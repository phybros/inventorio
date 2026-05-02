package main

import (
	"log"
	"net/http"
	"strconv"
)

// attrMergeValues returns human-readable display and form-submission raw values
// for a component attribute, suitable for the merge UI.
func attrMergeValues(a *ComponentAttribute) (display, raw string) {
	if a == nil {
		return "", ""
	}
	switch a.AttrDataType {
	case "numeric":
		if a.ValueNumeric != nil {
			u := ""
			if a.AttrUnit != nil {
				u = *a.AttrUnit
			}
			raw = FormatSIInput(*a.ValueNumeric, u)
			display = FormatSI(*a.ValueNumeric, u)
		}
	case "text":
		if a.ValueText != nil {
			raw = *a.ValueText
			display = *a.ValueText
		}
	case "enum":
		if a.ValueEnum != nil {
			raw = *a.ValueEnum
			if a.EnumDisplayName != nil {
				display = *a.EnumDisplayName
			} else {
				display = *a.ValueEnum
			}
		}
	case "boolean":
		if a.ValueBoolean != nil && *a.ValueBoolean {
			raw = "on"
			display = "Yes"
		}
	}
	return
}

// HandleMergeList renders the list of duplicate MPN groups.
func (app *App) HandleMergeList(w http.ResponseWriter, r *http.Request) {
	groups, err := findDuplicateComponents(app.db)
	if err != nil {
		log.Printf("error finding duplicates: %v", err)
		http.Error(w, "failed to load duplicates", http.StatusInternalServerError)
		return
	}
	app.renderer.RenderPage(w, r, "components/merge_list", mergeListPage{Groups: groups})
}

// HandleMergePreview renders the side-by-side merge UI for two specific components.
func (app *App) HandleMergePreview(w http.ResponseWriter, r *http.Request) {
	aID := r.URL.Query().Get("a")
	bID := r.URL.Query().Get("b")
	if aID == "" || bID == "" {
		http.Redirect(w, r, "/components/merge", http.StatusSeeOther)
		return
	}

	compA, err := getComponent(app.db, aID)
	if err != nil {
		log.Printf("error fetching component A: %v", err)
		http.Error(w, "component A not found", http.StatusNotFound)
		return
	}
	compB, err := getComponent(app.db, bID)
	if err != nil {
		log.Printf("error fetching component B: %v", err)
		http.Error(w, "component B not found", http.StatusNotFound)
		return
	}

	attrsA, err := getComponentAttributes(app.db, aID)
	if err != nil {
		log.Printf("error fetching attrs A: %v", err)
		http.Error(w, "failed to load attributes", http.StatusInternalServerError)
		return
	}
	attrsB, err := getComponentAttributes(app.db, bID)
	if err != nil {
		log.Printf("error fetching attrs B: %v", err)
		http.Error(w, "failed to load attributes", http.StatusInternalServerError)
		return
	}

	// Build the union of attribute definitions from both components' categories.
	defsA, err := getMergedAttributes(app.db, compA.CategoryID)
	if err != nil {
		log.Printf("error fetching defs A: %v", err)
		http.Error(w, "failed to load attribute definitions", http.StatusInternalServerError)
		return
	}
	defsB, err := getMergedAttributes(app.db, compB.CategoryID)
	if err != nil {
		log.Printf("error fetching defs B: %v", err)
		http.Error(w, "failed to load attribute definitions", http.StatusInternalServerError)
		return
	}

	defMap := make(map[string]AttributeDefinition)
	var defOrder []string
	for _, d := range defsA {
		if _, ok := defMap[d.ID]; !ok {
			defMap[d.ID] = d
			defOrder = append(defOrder, d.ID)
		}
	}
	for _, d := range defsB {
		if _, ok := defMap[d.ID]; !ok {
			defMap[d.ID] = d
			defOrder = append(defOrder, d.ID)
		}
	}

	// Index each component's attributes by definition ID.
	attrAByDef := make(map[string]*ComponentAttribute, len(attrsA))
	for i := range attrsA {
		attrAByDef[attrsA[i].AttributeDefinitionID] = &attrsA[i]
	}
	attrBByDef := make(map[string]*ComponentAttribute, len(attrsB))
	for i := range attrsB {
		attrBByDef[attrsB[i].AttributeDefinitionID] = &attrsB[i]
	}

	var attrRows []mergeAttrRow
	for _, defID := range defOrder {
		def := defMap[defID]
		attrA := attrAByDef[defID]
		attrB := attrBByDef[defID]

		dispA, rawA := attrMergeValues(attrA)
		dispB, rawB := attrMergeValues(attrB)

		defaultVal := rawA
		if defaultVal == "" {
			defaultVal = rawB
		}

		row := mergeAttrRow{
			Def:      def,
			AttrA:    attrA,
			AttrB:    attrB,
			DisplayA: dispA,
			DisplayB: dispB,
			RawA:     rawA,
			RawB:     rawB,
			Default:  defaultVal,
		}
		if def.DataType == "enum" && def.EnumGroupID != nil {
			vals, err := getEnumValuesForGroup(app.db, *def.EnumGroupID)
			if err != nil {
				log.Printf("error fetching enum values for attr %s: %v", def.ID, err)
			} else {
				row.EnumValues = vals
			}
		}
		attrRows = append(attrRows, row)
	}

	categories, err := listAllCategories(app.db)
	if err != nil {
		log.Printf("error listing categories: %v", err)
		http.Error(w, "failed to load categories", http.StatusInternalServerError)
		return
	}
	locations, err := listStorageLocations(app.db)
	if err != nil {
		log.Printf("error listing locations: %v", err)
		http.Error(w, "failed to load locations", http.StatusInternalServerError)
		return
	}

	app.renderer.RenderPage(w, r, "components/merge", mergePage{
		CompA:      compA,
		CompB:      compB,
		AttrRows:   attrRows,
		Categories: categories,
		Locations:  locations,
		SumQty:     compA.Quantity + compB.Quantity,
	})
}

// HandleMergeCommit processes the merge form submission. It updates the survivor
// component with the chosen field values, reassigns any BOM references from the
// loser to the survivor, and then deletes the loser.
func (app *App) HandleMergeCommit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	survivorID := r.FormValue("survivor_id")
	loserID := r.FormValue("loser_id")
	if survivorID == "" || loserID == "" {
		http.Error(w, "missing survivor_id or loser_id", http.StatusBadRequest)
		return
	}

	existing, err := getComponent(app.db, survivorID)
	if err != nil {
		http.Error(w, "survivor component not found", http.StatusNotFound)
		return
	}

	categoryID := r.FormValue("category_id")
	if categoryID == "" {
		categoryID = existing.CategoryID
	}

	qty, _ := strconv.Atoi(r.FormValue("quantity"))
	minQty, _ := strconv.Atoi(r.FormValue("min_quantity"))

	updated := &Component{
		ID:          survivorID,
		CategoryID:  categoryID,
		Quantity:    qty,
		MinQuantity: minQty,
	}
	if v := r.FormValue("mpn"); v != "" {
		updated.MPN = &v
	}
	if v := r.FormValue("manufacturer"); v != "" {
		updated.Manufacturer = &v
	}
	if v := r.FormValue("description"); v != "" {
		updated.Description = &v
	}
	if v := r.FormValue("location_id"); v != "" {
		updated.LocationID = &v
	}
	if v := r.FormValue("datasheet_url"); v != "" {
		updated.DatasheetURL = &v
	}
	if v := r.FormValue("notes"); v != "" {
		updated.Notes = &v
	}

	attrs, err := app.parseAttrValues(r, categoryID)
	if err != nil {
		log.Printf("error parsing attributes: %v", err)
		http.Error(w, "failed to parse attributes", http.StatusInternalServerError)
		return
	}

	tx, err := app.db.Begin()
	if err != nil {
		log.Printf("error beginning transaction: %v", err)
		http.Error(w, "failed to begin transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if err := updateComponent(tx, updated); err != nil {
		log.Printf("error updating survivor: %v", err)
		http.Error(w, "failed to update component", http.StatusInternalServerError)
		return
	}
	if err := saveComponentAttributes(tx, survivorID, attrs); err != nil {
		log.Printf("error saving attributes: %v", err)
		http.Error(w, "failed to save attributes", http.StatusInternalServerError)
		return
	}
	if err := reassignBOMItems(tx, loserID, survivorID); err != nil {
		log.Printf("error reassigning BOM items: %v", err)
		http.Error(w, "failed to reassign BOM items", http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec(`DELETE FROM components WHERE id = $1`, loserID); err != nil {
		log.Printf("error deleting loser: %v", err)
		http.Error(w, "failed to delete duplicate", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("error committing merge: %v", err)
		http.Error(w, "failed to commit merge", http.StatusInternalServerError)
		return
	}

	insertAuditLog(r, app.db, "components", survivorID, "merge",
		map[string]string{"merged_from": loserID}, updated)

	http.Redirect(w, r, "/components/"+survivorID, http.StatusSeeOther)
}
