package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
)

type filterFieldData struct {
	Attr       AttributeDefinition
	EnumValues []EnumValue
	// Current filter values for preserving state
	MinValue   string
	MaxValue   string
	SelEnums   map[string]bool
	TextValue  string
	BoolValue  string // "on" or ""
}

type componentListData struct {
	Components  []ComponentListItem
	Categories  []CategoryListItem
	CategoryID  string
	SearchQuery string
	Filters     []filterFieldData
}

func (app *App) HandleComponentList(w http.ResponseWriter, r *http.Request) {
	categoryID := r.URL.Query().Get("category")
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))

	var filters []AttrFilter
	var filterFields []filterFieldData

	// If a category is selected, load its attribute definitions and parse filters
	if categoryID != "" {
		attrDefs, err := getMergedAttributes(app.db, categoryID)
		if err != nil {
			log.Printf("error loading attribute defs: %v", err)
			http.Error(w, "failed to load attributes", http.StatusInternalServerError)
			return
		}

		for _, ad := range attrDefs {
			fd := filterFieldData{Attr: ad}
			var f AttrFilter
			f.AttrDefID = ad.ID
			f.DataType = ad.DataType
			hasFilter := false

			switch ad.DataType {
			case "numeric":
				if v := r.URL.Query().Get("min_" + ad.ID); v != "" {
					fd.MinValue = v
					if n, err := ParseSI(v); err == nil {
						f.MinValue = &n
						hasFilter = true
					}
				}
				if v := r.URL.Query().Get("max_" + ad.ID); v != "" {
					fd.MaxValue = v
					if n, err := ParseSI(v); err == nil {
						f.MaxValue = &n
						hasFilter = true
					}
				}
			case "enum":
				if ad.EnumGroupID != nil {
					vals, err := getEnumValuesForGroup(app.db, *ad.EnumGroupID)
					if err == nil {
						fd.EnumValues = vals
					}
				}
				fd.SelEnums = make(map[string]bool)
				if selected, ok := r.URL.Query()["enum_"+ad.ID]; ok && len(selected) > 0 {
					// Handle both multi-value params and comma-separated
					var allVals []string
					for _, s := range selected {
						allVals = append(allVals, strings.Split(s, ",")...)
					}
					for _, s := range allVals {
						if s != "" {
							fd.SelEnums[s] = true
						}
					}
					f.EnumValues = allVals
					hasFilter = len(allVals) > 0
				}
			case "text":
				if v := r.URL.Query().Get("text_" + ad.ID); v != "" {
					fd.TextValue = v
					f.TextSearch = v
					hasFilter = true
				}
			case "boolean":
				if v := r.URL.Query().Get("bool_" + ad.ID); v == "on" {
					fd.BoolValue = "on"
					b := true
					f.BoolValue = &b
					hasFilter = true
				}
			}

			filterFields = append(filterFields, fd)
			if hasFilter {
				filters = append(filters, f)
			}
		}
	}

	components, err := listComponents(app.db, categoryID, searchQuery, filters)
	if err != nil {
		log.Printf("error listing components: %v", err)
		http.Error(w, "failed to load components", http.StatusInternalServerError)
		return
	}

	categories, err := listAllCategories(app.db)
	if err != nil {
		log.Printf("error listing categories: %v", err)
		http.Error(w, "failed to load categories", http.StatusInternalServerError)
		return
	}

	app.renderer.RenderPage(w, "components/list", componentListData{
		Components:  components,
		Categories:  categories,
		CategoryID:  categoryID,
		SearchQuery: searchQuery,
		Filters:     filterFields,
	})
}

type componentDetailData struct {
	Component  *Component
	Attributes []ComponentAttribute
	Components []ComponentListItem // for merge picker
}

func (app *App) HandleComponentDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	comp, err := getComponent(app.db, id)
	if err != nil {
		log.Printf("error fetching component: %v", err)
		http.Error(w, "component not found", http.StatusNotFound)
		return
	}

	attrs, err := getComponentAttributes(app.db, id)
	if err != nil {
		log.Printf("error fetching component attributes: %v", err)
		http.Error(w, "failed to load attributes", http.StatusInternalServerError)
		return
	}

	allComps, err := listAllComponents(app.db)
	if err != nil {
		log.Printf("error listing components for merge picker: %v", err)
		http.Error(w, "failed to load components", http.StatusInternalServerError)
		return
	}

	app.renderer.RenderPage(w, "components/detail", componentDetailData{
		Component:  comp,
		Attributes: attrs,
		Components: allComps,
	})
}

type attrFieldData struct {
	Attr       AttributeDefinition
	EnumValues []EnumValue
	Value      *ComponentAttribute // nil for new components
}

type componentFormData struct {
	Component  *Component
	Categories []CategoryListItem
	Locations  []StorageLocation
	Fields     []attrFieldData
	IsNew      bool
	Errors     map[string]string
}

func (app *App) HandleComponentNew(w http.ResponseWriter, r *http.Request) {
	categoryID := r.URL.Query().Get("category")

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

	data := componentFormData{
		Categories: categories,
		Locations:  locations,
		IsNew:      true,
	}

	if categoryID != "" {
		fields, err := app.buildAttrFields(categoryID, nil)
		if err != nil {
			log.Printf("error building attr fields: %v", err)
			http.Error(w, "failed to load attribute fields", http.StatusInternalServerError)
			return
		}
		data.Fields = fields
		data.Component = &Component{CategoryID: categoryID}
	}

	app.renderer.RenderPage(w, "components/form", data)
}

// renderNewComponentForm re-renders the new component form with the given errors
// and the values the user already entered, so nothing is lost on validation failure.
func (app *App) renderNewComponentForm(w http.ResponseWriter, r *http.Request, errors map[string]string) {
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

	qty, _ := strconv.Atoi(r.FormValue("quantity"))
	minQty, _ := strconv.Atoi(r.FormValue("min_quantity"))
	comp := &Component{
		CategoryID:  r.FormValue("category_id"),
		Quantity:    qty,
		MinQuantity: minQty,
	}
	if v := r.FormValue("mpn"); v != "" {
		comp.MPN = &v
	}
	if v := r.FormValue("manufacturer"); v != "" {
		comp.Manufacturer = &v
	}
	if v := r.FormValue("description"); v != "" {
		comp.Description = &v
	}
	if v := r.FormValue("location_id"); v != "" {
		comp.LocationID = &v
	}
	if v := r.FormValue("datasheet_url"); v != "" {
		comp.DatasheetURL = &v
	}
	if v := r.FormValue("notes"); v != "" {
		comp.Notes = &v
	}

	var fields []attrFieldData
	if comp.CategoryID != "" {
		fields, _ = app.buildAttrFields(comp.CategoryID, nil)
	}

	w.WriteHeader(http.StatusUnprocessableEntity)
	app.renderer.RenderPage(w, "components/form", componentFormData{
		Component:  comp,
		Categories: categories,
		Locations:  locations,
		Fields:     fields,
		IsNew:      true,
		Errors:     errors,
	})
}

func (app *App) HandleComponentCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	categoryID := r.FormValue("category_id")
	if categoryID == "" {
		app.renderNewComponentForm(w, r, map[string]string{"category_id": "Category is required"})
		return
	}

	qty, _ := strconv.Atoi(r.FormValue("quantity"))
	minQty, _ := strconv.Atoi(r.FormValue("min_quantity"))

	comp := &Component{
		CategoryID:  categoryID,
		Quantity:    qty,
		MinQuantity: minQty,
	}
	if v := r.FormValue("mpn"); v != "" {
		comp.MPN = &v
	}
	if v := r.FormValue("manufacturer"); v != "" {
		comp.Manufacturer = &v
	}
	if v := r.FormValue("description"); v != "" {
		comp.Description = &v
	}
	if v := r.FormValue("location_id"); v != "" {
		comp.LocationID = &v
	}
	if v := r.FormValue("datasheet_url"); v != "" {
		comp.DatasheetURL = &v
	}
	if v := r.FormValue("notes"); v != "" {
		comp.Notes = &v
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
		http.Error(w, "failed to create component", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	id, err := createComponent(tx, comp)
	if err != nil {
		log.Printf("error creating component: %v", err)
		http.Error(w, "failed to create component", http.StatusInternalServerError)
		return
	}
	if err := saveComponentAttributes(tx, id, attrs); err != nil {
		log.Printf("error saving attributes: %v", err)
		http.Error(w, "failed to save attributes", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("error committing transaction: %v", err)
		http.Error(w, "failed to create component", http.StatusInternalServerError)
		return
	}

	insertAuditLog(app.db, "components", id, "insert", nil, comp)

	http.Redirect(w, r, "/components/"+id, http.StatusSeeOther)
}

func (app *App) HandleComponentEdit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	comp, err := getComponent(app.db, id)
	if err != nil {
		log.Printf("error fetching component: %v", err)
		http.Error(w, "component not found", http.StatusNotFound)
		return
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

	existingAttrs, err := getComponentAttributes(app.db, id)
	if err != nil {
		log.Printf("error fetching attributes: %v", err)
		http.Error(w, "failed to load attributes", http.StatusInternalServerError)
		return
	}

	fields, err := app.buildAttrFields(comp.CategoryID, existingAttrs)
	if err != nil {
		log.Printf("error building attr fields: %v", err)
		http.Error(w, "failed to load attribute fields", http.StatusInternalServerError)
		return
	}

	app.renderer.RenderPage(w, "components/form", componentFormData{
		Component:  comp,
		Categories: categories,
		Locations:  locations,
		Fields:     fields,
		IsNew:      false,
	})
}

func (app *App) HandleComponentUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	existing, err := getComponent(app.db, id)
	if err != nil {
		http.Error(w, "component not found", http.StatusNotFound)
		return
	}

	qty, _ := strconv.Atoi(r.FormValue("quantity"))
	minQty, _ := strconv.Atoi(r.FormValue("min_quantity"))

	categoryID := r.FormValue("category_id")
	if categoryID == "" {
		categoryID = existing.CategoryID
	}

	comp := &Component{
		ID:          id,
		CategoryID:  categoryID,
		Quantity:    qty,
		MinQuantity: minQty,
	}
	if v := r.FormValue("mpn"); v != "" {
		comp.MPN = &v
	}
	if v := r.FormValue("manufacturer"); v != "" {
		comp.Manufacturer = &v
	}
	if v := r.FormValue("description"); v != "" {
		comp.Description = &v
	}
	if v := r.FormValue("location_id"); v != "" {
		comp.LocationID = &v
	}
	if v := r.FormValue("datasheet_url"); v != "" {
		comp.DatasheetURL = &v
	}
	if v := r.FormValue("notes"); v != "" {
		comp.Notes = &v
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
		http.Error(w, "failed to update component", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if err := updateComponent(tx, comp); err != nil {
		log.Printf("error updating component: %v", err)
		http.Error(w, "failed to update component", http.StatusInternalServerError)
		return
	}
	if err := saveComponentAttributes(tx, id, attrs); err != nil {
		log.Printf("error saving attributes: %v", err)
		http.Error(w, "failed to save attributes", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("error committing transaction: %v", err)
		http.Error(w, "failed to update component", http.StatusInternalServerError)
		return
	}

	insertAuditLog(app.db, "components", id, "update", existing, comp)

	http.Redirect(w, r, "/components/"+id, http.StatusSeeOther)
}

func (app *App) HandleComponentDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	existing, _ := getComponent(app.db, id)

	if err := deleteComponent(app.db, id); err != nil {
		log.Printf("error deleting component: %v", err)
		http.Error(w, "failed to delete component", http.StatusInternalServerError)
		return
	}

	insertAuditLog(app.db, "components", id, "delete", existing, nil)

	w.Header().Set("HX-Redirect", "/components")
	w.WriteHeader(http.StatusOK)
}

func (app *App) HandleComponentQuantity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	delta, err := strconv.Atoi(r.FormValue("delta"))
	if err != nil {
		http.Error(w, "invalid delta", http.StatusBadRequest)
		return
	}

	oldQty := 0
	if existing, err := getComponent(app.db, id); err == nil {
		oldQty = existing.Quantity
	}

	newQty, err := adjustComponentQuantity(app.db, id, delta)
	if err != nil {
		log.Printf("error adjusting quantity: %v", err)
		http.Error(w, "failed to adjust quantity", http.StatusInternalServerError)
		return
	}

	insertAuditLog(app.db, "components", id, "quantity",
		map[string]int{"quantity": oldQty},
		map[string]int{"quantity": newQty})

	app.renderer.RenderFragment(w, "components/_quantity", map[string]any{
		"ID":       id,
		"Quantity": newQty,
	})
}

// HandleComponentAttrFields returns the dynamic attribute fields for a category (HTMX fragment).
func (app *App) HandleComponentAttrFields(w http.ResponseWriter, r *http.Request) {
	categoryID := r.URL.Query().Get("category_id")
	if categoryID == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	fields, err := app.buildAttrFields(categoryID, nil)
	if err != nil {
		log.Printf("error building attr fields: %v", err)
		http.Error(w, "failed to load attribute fields", http.StatusInternalServerError)
		return
	}

	app.renderer.RenderFragment(w, "components/_attr_fields", fields)
}

// buildAttrFields constructs the form field data for a category's attributes.
func (app *App) buildAttrFields(categoryID string, existing []ComponentAttribute) ([]attrFieldData, error) {
	attrDefs, err := getMergedAttributes(app.db, categoryID)
	if err != nil {
		return nil, err
	}

	// Index existing values by attribute_definition_id
	existingByDef := make(map[string]*ComponentAttribute)
	for i := range existing {
		existingByDef[existing[i].AttributeDefinitionID] = &existing[i]
	}

	var fields []attrFieldData
	for _, ad := range attrDefs {
		fd := attrFieldData{
			Attr:  ad,
			Value: existingByDef[ad.ID],
		}

		// Load enum values if this is an enum attribute
		if ad.DataType == "enum" && ad.EnumGroupID != nil {
			vals, err := getEnumValuesForGroup(app.db, *ad.EnumGroupID)
			if err != nil {
				return nil, err
			}
			fd.EnumValues = vals
		}

		fields = append(fields, fd)
	}

	return fields, nil
}

// parseAttrValues extracts attribute values from the form for a given category.
func (app *App) parseAttrValues(r *http.Request, categoryID string) ([]ComponentAttribute, error) {
	attrDefs, err := getMergedAttributes(app.db, categoryID)
	if err != nil {
		return nil, err
	}

	var attrs []ComponentAttribute
	for _, ad := range attrDefs {
		formKey := "attr_" + ad.ID
		val := r.FormValue(formKey)
		if val == "" {
			continue
		}

		ca := ComponentAttribute{
			AttributeDefinitionID: ad.ID,
		}

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
