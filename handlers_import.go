package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	xls "github.com/extrame/xls"
)

// ImportRow holds one parsed line from a DigiKey or Mouser export.
type ImportRow struct {
	MPN          string
	Manufacturer string
	Description  string
	Quantity     int
	MinQuantity  int
	LocationID   string
	DatasheetURL string
	Notes        string
	AttrFields   []importAttrFieldData
	CategoryHint string // Customer Reference (DigiKey) or empty (Mouser) — raw text from file
	CategoryID   string // matched existing category ID (populated during preview)
	SourceRef    string // DigiKey Part # or Mouser Part #
	// Populated during preview if a matching component exists.
	ExistingID   string
	ExistingQty  int
	ExistingMfr  string
	ExistingDesc string
}

func (r ImportRow) IsMatch() bool { return r.ExistingID != "" }

type importPageData struct {
	Categories []CategoryListItem
}

type importPreviewData struct {
	Rows        []ImportRow
	Categories  []CategoryListItem
	Locations   []StorageLocation
	Format      string
	ParseErrors []string
}

type importAttrFieldData struct {
	Attr       AttributeDefinition
	EnumValues []EnumValue
	Value      *ComponentAttribute
	InputName  string
}

type importAttrFieldsData struct {
	Fields []importAttrFieldData
}

type importResultData struct {
	Imported int
	Updated  int
	Skipped  int
	Errors   []string
}

func (app *App) HandleImportPage(w http.ResponseWriter, r *http.Request) {
	categories, err := listAllCategories(app.db)
	if err != nil {
		log.Printf("error loading categories: %v", err)
		http.Error(w, "failed to load categories", http.StatusInternalServerError)
		return
	}
	app.renderer.RenderPage(w, r, "import/page", importPageData{Categories: categories})
}

func (app *App) HandleImportPreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	format := r.FormValue("format") // "digikey" or "mouser"
	if format != "digikey" && format != "mouser" {
		http.Error(w, "invalid format", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "no file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	var rows []ImportRow
	var parseErrors []string

	switch format {
	case "digikey":
		rows, parseErrors, err = parseDigiKeyCSV(file)
		if err != nil {
			http.Error(w, "failed to parse DigiKey CSV: "+err.Error(), http.StatusBadRequest)
			return
		}
	case "mouser":
		tmp, tmpErr := os.CreateTemp("", "mouser-*.xls")
		if tmpErr != nil {
			http.Error(w, "failed to create temp file", http.StatusInternalServerError)
			return
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName)
		if _, copyErr := io.Copy(tmp, file); copyErr != nil {
			tmp.Close()
			http.Error(w, "failed to save uploaded file", http.StatusInternalServerError)
			return
		}
		tmp.Close()
		rows, parseErrors, err = parseMouserXLS(tmpName)
		if err != nil {
			http.Error(w, "failed to parse Mouser XLS: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	categories, err := listAllCategories(app.db)
	if err != nil {
		log.Printf("error loading categories: %v", err)
		http.Error(w, "failed to load categories", http.StatusInternalServerError)
		return
	}
	locations, err := listStorageLocations(app.db)
	if err != nil {
		log.Printf("error loading locations: %v", err)
		http.Error(w, "failed to load locations", http.StatusInternalServerError)
		return
	}

	// Build a case-insensitive name→ID index for category matching.
	catByName := make(map[string]string)
	for _, c := range categories {
		catByName[strings.ToLower(strings.TrimSpace(c.Name))] = c.ID
	}

	// Match each row: MPN dedup + category hint → ID.
	for i := range rows {
		if hint := strings.ToLower(strings.TrimSpace(rows[i].CategoryHint)); hint != "" {
			rows[i].CategoryID = catByName[hint] // empty string if no match
		}
		if rows[i].CategoryID == "" {
			rows[i].CategoryID = suggestCategoryID(rows[i].Description, categories)
		}
		if rows[i].MPN == "" {
			continue
		}
		existing, err := findComponentByMPN(app.db, rows[i].MPN)
		if err != nil || existing == nil {
			if rows[i].CategoryID != "" {
				fields, err := app.buildImportAttrFields(rows[i].CategoryID, "attr_"+strconv.Itoa(i)+"_", rows[i].Description, nil)
				if err != nil {
					log.Printf("error loading suggested import attrs: %v", err)
				} else {
					rows[i].AttrFields = fields
				}
			}
			continue
		}
		rows[i].ExistingID = existing.ID
		rows[i].ExistingQty = existing.Quantity
		rows[i].CategoryID = existing.CategoryID
		rows[i].MinQuantity = existing.MinQuantity
		if existing.LocationID != nil {
			rows[i].LocationID = *existing.LocationID
		}
		if existing.DatasheetURL != nil {
			rows[i].DatasheetURL = *existing.DatasheetURL
		}
		if existing.Notes != nil {
			rows[i].Notes = *existing.Notes
		}
		if existing.Manufacturer != nil {
			rows[i].ExistingMfr = *existing.Manufacturer
			if rows[i].Manufacturer == "" {
				rows[i].Manufacturer = *existing.Manufacturer
			}
		}
		if existing.Description != nil {
			rows[i].ExistingDesc = *existing.Description
			if rows[i].Description == "" {
				rows[i].Description = *existing.Description
			}
		}
		existingAttrs, err := getComponentAttributes(app.db, existing.ID)
		if err != nil {
			log.Printf("error loading existing import attrs: %v", err)
			existingAttrs = nil
		}
		fields, err := app.buildImportAttrFields(rows[i].CategoryID, "attr_"+strconv.Itoa(i)+"_", rows[i].Description, existingAttrs)
		if err != nil {
			log.Printf("error loading import attrs: %v", err)
		} else {
			rows[i].AttrFields = fields
		}
	}

	app.renderer.RenderFragment(w, "import/_preview", importPreviewData{
		Rows:        rows,
		Categories:  categories,
		Locations:   locations,
		Format:      format,
		ParseErrors: parseErrors,
	})
}

func (app *App) HandleImportAttrFields(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	row := r.URL.Query().Get("row")
	categoryID := r.FormValue("cat_id_" + row)
	if categoryID == "" {
		categoryID = r.FormValue("category_id")
	}
	if categoryID == "" {
		app.renderer.RenderFragment(w, "import/_attr_fields", importAttrFieldsData{})
		return
	}

	desc := r.FormValue("desc_" + row)
	fields, err := app.buildImportAttrFields(categoryID, "attr_"+row+"_", desc, nil)
	if err != nil {
		log.Printf("error loading import attributes: %v", err)
		http.Error(w, "failed to load attributes", http.StatusInternalServerError)
		return
	}
	app.renderer.RenderFragment(w, "import/_attr_fields", importAttrFieldsData{Fields: fields})
}

func (app *App) HandleImportCommit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	count, _ := strconv.Atoi(r.FormValue("count"))
	defaultCatID := r.FormValue("default_category_id")

	var imported, updated, skipped int
	var errors []string

	for i := range count {
		idx := strconv.Itoa(i)

		if r.FormValue("include_"+idx) != "1" {
			skipped++
			continue
		}

		mpn := strings.TrimSpace(r.FormValue("mpn_" + idx))
		mfr := strings.TrimSpace(r.FormValue("mfr_" + idx))
		desc := strings.TrimSpace(r.FormValue("desc_" + idx))
		qty, _ := strconv.Atoi(r.FormValue("qty_" + idx))
		minQty, _ := strconv.Atoi(r.FormValue("min_qty_" + idx))
		catID := r.FormValue("cat_id_" + idx)
		existingID := r.FormValue("existing_id_" + idx)
		locationID := strings.TrimSpace(r.FormValue("location_id_" + idx))
		datasheetURL := strings.TrimSpace(r.FormValue("datasheet_url_" + idx))
		notes := strings.TrimSpace(r.FormValue("notes_" + idx))

		if catID == "" {
			catID = defaultCatID
		}

		if existingID != "" {
			existing, err := getComponent(app.db, existingID)
			if err != nil {
				errors = append(errors, fmt.Sprintf("row %d (%s): existing component not found: %v", i+1, mpn, err))
				skipped++
				continue
			}
			if catID == "" {
				catID = existing.CategoryID
			}

			comp := &Component{
				ID:           existingID,
				CategoryID:   catID,
				Quantity:     existing.Quantity + qty,
				MinQuantity:  minQty,
				MPN:          stringPtrIfNotEmpty(mpn),
				Manufacturer: stringPtrIfNotEmpty(mfr),
				Description:  stringPtrIfNotEmpty(desc),
				LocationID:   stringPtrIfNotEmpty(locationID),
				DatasheetURL: stringPtrIfNotEmpty(datasheetURL),
				Notes:        stringPtrIfNotEmpty(notes),
			}
			attrs, err := app.parseAttrValuesWithPrefix(r, catID, "attr_"+idx+"_")
			if err != nil {
				errors = append(errors, fmt.Sprintf("row %d (%s): attributes failed: %v", i+1, mpn, err))
				skipped++
				continue
			}

			tx, err := app.db.Begin()
			if err != nil {
				errors = append(errors, fmt.Sprintf("row %d (%s): merge failed: %v", i+1, mpn, err))
				skipped++
				continue
			}
			if err := updateComponent(tx, comp); err != nil {
				tx.Rollback()
				errors = append(errors, fmt.Sprintf("row %d (%s): merge failed: %v", i+1, mpn, err))
				skipped++
				continue
			}
			if err := saveComponentAttributes(tx, existingID, attrs); err != nil {
				tx.Rollback()
				errors = append(errors, fmt.Sprintf("row %d (%s): attribute save failed: %v", i+1, mpn, err))
				skipped++
				continue
			}
			if err := tx.Commit(); err != nil {
				errors = append(errors, fmt.Sprintf("row %d (%s): merge commit failed: %v", i+1, mpn, err))
				skipped++
				continue
			}
			insertAuditLog(r, app.db, "components", existingID, "import-merge", nil,
				map[string]any{"qty_added": qty, "mpn": mpn, "edited": true})
			updated++
			continue
		}

		// New component — category ID must be set.
		if catID == "" {
			errors = append(errors, fmt.Sprintf("row %d (%s): no category assigned", i+1, mpn))
			skipped++
			continue
		}

		comp := &Component{
			CategoryID:   catID,
			Quantity:     qty,
			MinQuantity:  minQty,
			MPN:          stringPtrIfNotEmpty(mpn),
			Manufacturer: stringPtrIfNotEmpty(mfr),
			Description:  stringPtrIfNotEmpty(desc),
			LocationID:   stringPtrIfNotEmpty(locationID),
			DatasheetURL: stringPtrIfNotEmpty(datasheetURL),
			Notes:        stringPtrIfNotEmpty(notes),
		}
		attrs, err := app.parseAttrValuesWithPrefix(r, catID, "attr_"+idx+"_")
		if err != nil {
			errors = append(errors, fmt.Sprintf("row %d (%s): attributes failed: %v", i+1, mpn, err))
			skipped++
			continue
		}

		tx, err := app.db.Begin()
		if err != nil {
			errors = append(errors, fmt.Sprintf("row %d (%s): %v", i+1, mpn, err))
			skipped++
			continue
		}
		id, err := createComponent(tx, comp)
		if err != nil {
			tx.Rollback()
			errors = append(errors, fmt.Sprintf("row %d (%s): %v", i+1, mpn, err))
			skipped++
			continue
		}
		if err := saveComponentAttributes(tx, id, attrs); err != nil {
			tx.Rollback()
			errors = append(errors, fmt.Sprintf("row %d (%s): attribute save failed: %v", i+1, mpn, err))
			skipped++
			continue
		}
		if err := tx.Commit(); err != nil {
			errors = append(errors, fmt.Sprintf("row %d (%s): commit failed: %v", i+1, mpn, err))
			skipped++
			continue
		}
		insertAuditLog(r, app.db, "components", id, "import", nil, comp)
		imported++
	}

	app.renderer.RenderPage(w, r, "import/result", importResultData{
		Imported: imported,
		Updated:  updated,
		Skipped:  skipped,
		Errors:   errors,
	})
}

func stringPtrIfNotEmpty(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func (app *App) buildImportAttrFields(categoryID, prefix, desc string, existing []ComponentAttribute) ([]importAttrFieldData, error) {
	attrDefs, err := getMergedAttributes(app.db, categoryID)
	if err != nil {
		return nil, err
	}

	existingByDef := make(map[string]*ComponentAttribute)
	for i := range existing {
		existingByDef[existing[i].AttributeDefinitionID] = &existing[i]
	}

	var fields []importAttrFieldData
	for _, ad := range attrDefs {
		fd := importAttrFieldData{
			Attr:      ad,
			Value:     existingByDef[ad.ID],
			InputName: prefix + ad.ID,
		}
		if ad.DataType == "enum" && ad.EnumGroupID != nil {
			vals, err := getEnumValuesForGroup(app.db, *ad.EnumGroupID)
			if err != nil {
				return nil, err
			}
			fd.EnumValues = vals
		}
		if fd.Value == nil {
			if suggested := suggestAttrValue(desc, ad, fd.EnumValues); suggested != nil {
				fd.Value = suggested
			}
		}
		fields = append(fields, fd)
	}

	return fields, nil
}

func (app *App) parseAttrValuesWithPrefix(r *http.Request, categoryID, prefix string) ([]ComponentAttribute, error) {
	attrDefs, err := getMergedAttributes(app.db, categoryID)
	if err != nil {
		return nil, err
	}

	var attrs []ComponentAttribute
	for _, ad := range attrDefs {
		formKey := prefix + ad.ID
		val := strings.TrimSpace(r.FormValue(formKey))
		if val == "" {
			continue
		}

		ca := ComponentAttribute{AttributeDefinitionID: ad.ID}
		switch ad.DataType {
		case "numeric":
			f, err := ParseSI(val)
			if err != nil {
				continue
			}
			ca.ValueNumeric = &f
		case "text":
			ca.ValueText = &val
		case "enum":
			ca.ValueEnum = &val
		case "boolean":
			b := val == "on" || val == "true"
			ca.ValueBoolean = &b
		}
		attrs = append(attrs, ca)
	}
	return attrs, nil
}

func suggestCategoryID(desc string, categories []CategoryListItem) string {
	descCounts := descTokenCounts(desc)
	if len(descCounts) == 0 {
		return ""
	}
	tokens := tokenSet(descCounts)

	bestID := ""
	bestScore := 0
	for _, cat := range categories {
		pathTokens := descTokens(cat.Path)
		categoryTokens := descTokens(cat.Path + " " + derefString(cat.Description))
		score := strings.Count(cat.Path, "/") * 5
		for token, count := range descCounts {
			if _, ok := pathTokens[token]; ok {
				score += 30 + min(count, 3)*6
			} else if _, ok := categoryTokens[token]; ok {
				score += 16 + min(count, 3)*4
			} else if len(token) >= 3 && pathHasPrefixToken(pathTokens, token) {
				score += 12
			} else if len(token) >= 3 && pathHasPrefixToken(categoryTokens, token) {
				score += 8
			}
		}

		score += categoryFamilyScore(tokens, categoryTokens)

		if hasPackageToken(tokens) && hasAnyToken(pathTokens, "chip", "smd", "surface", "mount") {
			score += 20
		}
		if score > bestScore {
			bestScore = score
			bestID = cat.ID
		}
	}
	if bestScore < 50 {
		return ""
	}
	return bestID
}

func categoryFamilyScore(descTokens, categoryTokens map[string]struct{}) int {
	score := 0
	if hasAnyToken(descTokens, "cap", "capacitor") && hasAnyToken(categoryTokens, "capacitor", "capacitors") {
		score += 50
	}
	if hasAnyToken(descTokens, "res", "resistor") && hasAnyToken(categoryTokens, "resistor", "resistors") {
		score += 50
	}
	if hasAnyToken(descTokens, "xtal", "crystal", "osc", "oscillator") &&
		hasAnyToken(categoryTokens, "crystal", "crystals", "oscillator", "oscillators") {
		score += 50
	}
	if hasAnyToken(descTokens, "logic") && hasAnyToken(categoryTokens, "logic") {
		score += 45
	}
	if hasAnyToken(descTokens, "ic", "ics", "integrated", "circuit", "circuits") &&
		hasAnyToken(categoryTokens, "ic", "ics", "integrated", "circuit", "circuits") {
		score += 20
	}
	if hasAnyToken(descTokens, "bus", "line", "transceiver", "transceivers", "txrx") &&
		hasAnyToken(categoryTokens, "interface", "interfaces", "bus", "line", "transceiver", "transceivers") {
		score += 70
	}
	if hasAnyToken(descTokens, "inverter", "inverters", "invert", "schmitt") &&
		hasAnyToken(categoryTokens, "gate", "gates", "logic", "inverter", "inverters") {
		score += 35
	}
	if hasAnyToken(descTokens, "encoder", "encoders", "decoder", "decoders", "multiplexer", "multiplexers", "demultiplexer", "demultiplexers") &&
		hasAnyToken(categoryTokens, "logic", "gate", "gates", "encoder", "encoders", "decoder", "decoders", "multiplexer", "multiplexers", "demultiplexer", "demultiplexers") {
		score += 30
	}
	return score
}

func suggestAttrValue(desc string, attr AttributeDefinition, enumValues []EnumValue) *ComponentAttribute {
	if strings.TrimSpace(desc) == "" {
		return nil
	}
	ca := &ComponentAttribute{AttributeDefinitionID: attr.ID}

	switch attr.DataType {
	case "numeric":
		unit := ""
		if attr.Unit != nil {
			unit = *attr.Unit
		}
		raw := suggestNumericToken(desc, attr, unit)
		if raw == "" {
			return nil
		}
		val, err := ParseSI(raw)
		if err != nil {
			return nil
		}
		ca.ValueNumeric = &val
		return ca
	case "enum":
		id := suggestEnumValueID(desc, enumValues)
		if id == "" {
			return nil
		}
		ca.ValueEnum = &id
		return ca
	}
	return nil
}

func suggestNumericToken(desc string, attr AttributeDefinition, unit string) string {
	upper := strings.ToUpper(desc)
	label := normalizeToken(attr.Name + " " + attr.DisplayName)

	switch unit {
	case "F":
		if !strings.Contains(label, "capacitance") && !strings.Contains(upper, "CAP") {
			return ""
		}
		return normalizeNumericUnitValue(firstUnitValue(upper, `(?i)\b\d+(?:\.\d+)?\s*(?:PF|NF|UF|µF|μF|MF|F)\b`))
	case "Ω":
		if !strings.Contains(label, "resistance") && !strings.Contains(upper, "RES") {
			return ""
		}
		if v := firstUnitValue(upper, `(?i)\b\d+(?:\.\d+)?\s*(?:M\s*OHM|K\s*OHM|MOHM|KOHM|OHM|MΩ|KΩ|Ω)\b`); v != "" {
			return normalizeNumericUnitValue(v)
		}
	case "V":
		if strings.Contains(label, "voltage") || strings.Contains(label, "volt") {
			return suggestVoltageToken(upper, label)
		}
	case "W":
		if strings.Contains(label, "power") || strings.Contains(label, "watt") {
			if v := firstUnitValue(upper, `(?i)\b(?:\d+\s*/\s*\d+|\d+(?:\.\d+)?)\s*W\b`); v != "" {
				return normalizeNumericUnitValue(v)
			}
		}
	case "Hz":
		if strings.Contains(label, "frequency") || strings.Contains(label, "freq") {
			return normalizeNumericUnitValue(firstUnitValue(upper, `(?i)\b\d+(?:\.\d+)?\s*(?:GHZ|MHZ|KHZ|HZ)\b`))
		}
	}

	return ""
}

func suggestEnumValueID(desc string, values []EnumValue) string {
	tokens := descTokens(desc)
	if len(tokens) == 0 {
		return ""
	}

	bestID := ""
	bestScore := 0
	for _, v := range values {
		score := enumMatchScore(tokens, v.Value)
		if s := enumMatchScore(tokens, v.DisplayName); s > score {
			score = s
		}
		if score > bestScore {
			bestScore = score
			bestID = v.ID
		}
	}
	return bestID
}

func suggestVoltageToken(desc, label string) string {
	values := unitValues(desc, `(?i)\b\d+(?:\.\d+)?\s*V\b`)
	if len(values) == 0 {
		return ""
	}
	if strings.Contains(label, "min") {
		if len(values) >= 2 {
			return normalizeNumericUnitValue(values[0])
		}
		return ""
	}
	if strings.Contains(label, "max") {
		if len(values) >= 2 {
			return normalizeNumericUnitValue(values[len(values)-1])
		}
		return normalizeNumericUnitValue(values[0])
	}
	return normalizeNumericUnitValue(values[0])
}

func unitValues(s, pattern string) []string {
	re := regexp.MustCompile(pattern)
	matches := re.FindAllString(s, -1)
	for i := range matches {
		matches[i] = strings.TrimSpace(matches[i])
	}
	return matches
}

func enumMatchScore(desc map[string]struct{}, candidate string) int {
	candidateTokens := descTokens(candidate)
	score := 0
	for token := range candidateTokens {
		if _, ok := desc[token]; ok {
			score += 100
			continue
		}
		if len(token) >= 4 {
			if _, ok := desc[token[:3]]; ok {
				score += 75
			}
		}
		if strings.HasSuffix(token, "pct") && len(token) > 3 {
			if _, hasPct := desc["pct"]; hasPct {
				if _, hasNumber := desc[strings.TrimSuffix(token, "pct")]; hasNumber {
					score += 100
				}
			}
		}
	}
	return score
}

func firstUnitValue(s, pattern string) string {
	re := regexp.MustCompile(pattern)
	return strings.TrimSpace(re.FindString(s))
}

func normalizeNumericUnitValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, " ", "")
	upper := strings.ToUpper(s)
	if strings.HasSuffix(upper, "W") && strings.Contains(upper, "/") {
		fraction := strings.TrimSuffix(upper, "W")
		parts := strings.SplitN(fraction, "/", 2)
		if len(parts) == 2 {
			num, numErr := strconv.ParseFloat(parts[0], 64)
			den, denErr := strconv.ParseFloat(parts[1], 64)
			if numErr == nil && denErr == nil && den != 0 {
				return strconv.FormatFloat(num/den, 'g', 6, 64) + "W"
			}
		}
	}
	replacer := strings.NewReplacer(
		"PF", "pF",
		"NF", "nF",
		"UF", "uF",
		"µF", "uF",
		"μF", "uF",
		"MF", "mF",
		"MΩ", "MΩ",
		"KΩ", "kΩ",
		"MOHM", "MΩ",
		"KOHM", "kΩ",
		"M OHM", "MΩ",
		"K OHM", "kΩ",
		"OHM", "Ω",
		"GHZ", "GHz",
		"MHZ", "MHz",
		"KHZ", "kHz",
		"HZ", "Hz",
	)
	return replacer.Replace(upper)
}

func descTokens(s string) map[string]struct{} {
	tokens := make(map[string]struct{})
	for token := range descTokenCounts(s) {
		tokens[token] = struct{}{}
	}
	return tokens
}

func descTokenCounts(s string) map[string]int {
	counts := make(map[string]int)
	normalized := normalizeToken(s)
	for _, field := range strings.Fields(normalized) {
		for _, token := range tokenVariants(field) {
			counts[token]++
		}
	}
	return counts
}

func tokenSet(counts map[string]int) map[string]struct{} {
	tokens := make(map[string]struct{}, len(counts))
	for token := range counts {
		tokens[token] = struct{}{}
	}
	return tokens
}

func tokenVariants(token string) []string {
	if token == "" {
		return nil
	}

	var variants []string
	add := func(v string) {
		if v == "" {
			return
		}
		for _, existing := range variants {
			if existing == v {
				return
			}
		}
		variants = append(variants, v)
	}

	add(token)
	if strings.HasSuffix(token, "s") && len(token) > 3 {
		add(strings.TrimSuffix(token, "s"))
	}
	if strings.HasPrefix(token, "series") && len(token) > len("series") {
		add(strings.TrimPrefix(token, "series"))
	}
	for _, suffix := range []string{"transceiver", "inverter", "encoder", "decoder", "multiplexer", "demultiplexer"} {
		if strings.HasSuffix(token, suffix) && token != suffix {
			add(suffix)
		}
		if strings.HasSuffix(token, suffix+"s") && token != suffix+"s" {
			add(suffix)
			add(suffix + "s")
		}
	}
	if matches := regexp.MustCompile(`^([a-z]+)(\d+)$`).FindStringSubmatch(token); matches != nil {
		add(matches[1])
		add(matches[2])
	}

	return variants
}

func normalizeToken(s string) string {
	s = strings.ToLower(s)
	replacer := strings.NewReplacer(
		"µ", "u",
		"μ", "u",
		"Ω", " ohm ",
		"%", " pct ",
		"/", " ",
		"-", " ",
		"_", " ",
		".", " ",
		",", " ",
		"&", " ",
		"(", " ",
		")", " ",
		"\"", " ",
	)
	return replacer.Replace(s)
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func hasAnyToken(tokens map[string]struct{}, candidates ...string) bool {
	for _, candidate := range candidates {
		if _, ok := tokens[candidate]; ok {
			return true
		}
	}
	return false
}

func hasPackageToken(tokens map[string]struct{}) bool {
	for token := range tokens {
		if len(token) == 4 {
			if _, err := strconv.Atoi(token); err == nil {
				return true
			}
		}
	}
	return false
}

func pathHasPrefixToken(tokens map[string]struct{}, prefix string) bool {
	for token := range tokens {
		if strings.HasPrefix(token, prefix) {
			return true
		}
	}
	return false
}

// --- Parsers ---

func parseDigiKeyCSV(r io.Reader) ([]ImportRow, []string, error) {
	cr := csv.NewReader(r)
	cr.LazyQuotes = true

	headers, err := cr.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("reading headers: %w", err)
	}

	// Strip BOM if present on first header.
	if len(headers) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], "\xef\xbb\xbf")
		headers[0] = strings.TrimPrefix(headers[0], "\ufeff")
	}

	colIdx := make(map[string]int)
	for i, h := range headers {
		colIdx[strings.TrimSpace(h)] = i
	}

	get := func(row []string, col string) string {
		i, ok := colIdx[col]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	if _, ok := colIdx["Manufacturer Part Number"]; !ok {
		return nil, nil, fmt.Errorf("this doesn't look like a DigiKey CSV (missing 'Manufacturer Part Number' column). Found: %s", strings.Join(headers, ", "))
	}

	var rows []ImportRow
	var parseErrors []string
	lineNum := 1

	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		lineNum++
		if err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("line %d: %v", lineNum, err))
			continue
		}

		qty, _ := strconv.Atoi(get(record, "Quantity"))

		rows = append(rows, ImportRow{
			MPN:          get(record, "Manufacturer Part Number"),
			Manufacturer: get(record, "Manufacturer"),
			Description:  get(record, "Description"),
			Quantity:     qty,
			CategoryHint: get(record, "Customer Reference"),
			SourceRef:    get(record, "DigiKey Part #"),
		})
	}

	return rows, parseErrors, nil
}

var mouserPartNumRE = regexp.MustCompile(`^\d{3}-\S+`)

func parseMouserXLS(filename string) ([]ImportRow, []string, error) {
	wb, err := xls.Open(filename, "utf-8")
	if err != nil {
		return nil, nil, fmt.Errorf("opening XLS: %w", err)
	}

	sheet := wb.GetSheet(0)
	if sheet == nil {
		return nil, nil, fmt.Errorf("workbook has no sheets")
	}

	// Scan rows looking for a header row containing Mouser/Mfr column labels.
	colMouser := -1
	colMfr := -1
	colDesc := -1
	colQty := -1
	colMfrName := -1
	headerRow := -1

	maxRow := int(sheet.MaxRow)
	for ri := 0; ri <= maxRow && ri < 50; ri++ { // header won't be after row 50
		row := sheet.Row(ri)
		if row == nil {
			continue
		}
		for ci := 0; ci < row.LastCol(); ci++ {
			cell := strings.TrimSpace(row.Col(ci))
			cellLow := strings.ToLower(cell)
			switch {
			case strings.Contains(cellLow, "mouser") && strings.Contains(cellLow, "#"):
				colMouser = ci
				headerRow = ri
			case strings.Contains(cellLow, "mfr.") && strings.Contains(cellLow, "#"):
				colMfr = ci
			case cellLow == "desc." || cellLow == "description":
				colDesc = ci
			case strings.Contains(cellLow, "order qty") || cellLow == "qty":
				colQty = ci
			case cellLow == "manufacturer" || cellLow == "mfr. name":
				colMfrName = ci
			}
		}
		if headerRow >= 0 && colMfr >= 0 {
			break
		}
	}

	if headerRow < 0 {
		return nil, nil, fmt.Errorf("could not locate header row in Mouser XLS (looked for 'Mouser #' column in first 50 rows)")
	}
	if colMfr < 0 {
		return nil, nil, fmt.Errorf("could not locate 'Mfr. #' column in Mouser XLS")
	}

	var rows []ImportRow
	var parseErrors []string

	for ri := headerRow + 1; ri <= maxRow; ri++ {
		row := sheet.Row(ri)
		if row == nil {
			continue
		}

		getCell := func(col int) string {
			if col < 0 || col >= row.LastCol() {
				return ""
			}
			return strings.TrimSpace(row.Col(col))
		}

		mouserPart := getCell(colMouser)
		mfrPart := getCell(colMfr)

		// Skip rows that don't look like line items.
		if mouserPart == "" && mfrPart == "" {
			continue
		}
		if colMouser >= 0 && mouserPart != "" && !mouserPartNumRE.MatchString(mouserPart) {
			continue
		}

		qtyStr := getCell(colQty)
		// "10 Shipped" → "10"
		if fields := strings.Fields(qtyStr); len(fields) > 0 {
			qtyStr = fields[0]
		}
		qty, _ := strconv.Atoi(qtyStr)

		rows = append(rows, ImportRow{
			MPN:          mfrPart,
			Manufacturer: getCell(colMfrName),
			Description:  getCell(colDesc),
			Quantity:     qty,
			SourceRef:    mouserPart,
		})
	}

	if len(rows) == 0 {
		parseErrors = append(parseErrors, "no line items found after header row — the Mouser XLS layout may differ from expected")
	}

	return rows, parseErrors, nil
}
