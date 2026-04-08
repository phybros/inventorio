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
	Format      string
	ParseErrors []string
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
	app.renderer.RenderPage(w, "import/page", importPageData{Categories: categories})
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
		if rows[i].MPN == "" {
			continue
		}
		existing, err := findComponentByMPN(app.db, rows[i].MPN)
		if err != nil || existing == nil {
			continue
		}
		rows[i].ExistingID = existing.ID
		rows[i].ExistingQty = existing.Quantity
		if existing.Manufacturer != nil {
			rows[i].ExistingMfr = *existing.Manufacturer
		}
		if existing.Description != nil {
			rows[i].ExistingDesc = *existing.Description
		}
	}

	app.renderer.RenderFragment(w, "import/_preview", importPreviewData{
		Rows:        rows,
		Categories:  categories,
		Format:      format,
		ParseErrors: parseErrors,
	})
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
		catID := r.FormValue("cat_id_" + idx)
		existingID := r.FormValue("existing_id_" + idx)

		if catID == "" {
			catID = defaultCatID
		}

		if existingID != "" {
			// Merge into existing component: add qty, backfill blank fields.
			var mfrPtr, descPtr *string
			if mfr != "" {
				mfrPtr = &mfr
			}
			if desc != "" {
				descPtr = &desc
			}
			if err := importMergeComponent(app.db, existingID, qty, mfrPtr, descPtr); err != nil {
				errors = append(errors, fmt.Sprintf("row %d (%s): merge failed: %v", i+1, mpn, err))
				skipped++
				continue
			}
			insertAuditLog(app.db, "components", existingID, "import-merge", nil,
				map[string]any{"qty_added": qty, "mpn": mpn})
			updated++
			continue
		}

		// New component — category ID must be set.
		if catID == "" {
			errors = append(errors, fmt.Sprintf("row %d (%s): no category assigned", i+1, mpn))
			skipped++
			continue
		}

		comp := &Component{CategoryID: catID, Quantity: qty}
		if mpn != "" {
			comp.MPN = &mpn
		}
		if mfr != "" {
			comp.Manufacturer = &mfr
		}
		if desc != "" {
			comp.Description = &desc
		}

		id, err := createComponent(app.db, comp)
		if err != nil {
			errors = append(errors, fmt.Sprintf("row %d (%s): %v", i+1, mpn, err))
			skipped++
			continue
		}
		insertAuditLog(app.db, "components", id, "import", nil, comp)
		imported++
	}

	app.renderer.RenderPage(w, "import/result", importResultData{
		Imported: imported,
		Updated:  updated,
		Skipped:  skipped,
		Errors:   errors,
	})
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
