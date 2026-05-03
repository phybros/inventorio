package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type dashboardData struct {
	Stats    *DashboardStats
	LowStock []LowStockItem
	Recent   []ComponentListItem
}

func (app *App) HandleIndex(w http.ResponseWriter, r *http.Request) {
	stats, err := getDashboardStats(app.db)
	if err != nil {
		log.Printf("error fetching dashboard stats: %v", err)
		http.Error(w, "failed to load dashboard", http.StatusInternalServerError)
		return
	}

	lowStock, err := getLowStockComponents(app.db)
	if err != nil {
		log.Printf("error fetching low stock: %v", err)
		http.Error(w, "failed to load dashboard", http.StatusInternalServerError)
		return
	}

	recent, err := getRecentComponents(app.db, 10)
	if err != nil {
		log.Printf("error fetching recent components: %v", err)
		http.Error(w, "failed to load dashboard", http.StatusInternalServerError)
		return
	}

	app.renderer.RenderPage(w, r, "index", dashboardData{
		Stats:    stats,
		LowStock: lowStock,
		Recent:   recent,
	})
}

func (app *App) HandleCategoryList(w http.ResponseWriter, r *http.Request) {
	categories, err := listCategories(app.db)
	if err != nil {
		log.Printf("error listing categories: %v", err)
		http.Error(w, "failed to load categories", http.StatusInternalServerError)
		return
	}
	app.renderer.RenderPage(w, r, "categories/list", categories)
}

func (app *App) HandleCategoryCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	var parentID *string
	if pid := r.FormValue("parent_id"); pid != "" {
		parentID = &pid
	}

	var description *string
	if desc := r.FormValue("description"); desc != "" {
		description = &desc
	}

	id, err := createCategory(app.db, name, parentID, description)
	if err != nil {
		log.Printf("error creating category: %v", err)
		http.Error(w, "failed to create category", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/categories/"+id+"/edit", http.StatusSeeOther)
}

type categoryEditData struct {
	Category         *Category
	Parents          []Category
	Attributes       []AttributeDefinition
	EnumGroups       []EnumGroup
	IsNew            bool
}

func (app *App) HandleCategoryEdit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	cat, err := getCategory(app.db, id)
	if err != nil {
		log.Printf("error fetching category: %v", err)
		http.Error(w, "category not found", http.StatusNotFound)
		return
	}

	parents, err := listParentCandidates(app.db, id)
	if err != nil {
		log.Printf("error listing parent candidates: %v", err)
		http.Error(w, "failed to load data", http.StatusInternalServerError)
		return
	}

	attrs, err := listAttributesByCategory(app.db, id)
	if err != nil {
		log.Printf("error listing attributes: %v", err)
		http.Error(w, "failed to load attributes", http.StatusInternalServerError)
		return
	}

	enumGroups, err := listEnumGroupsSimple(app.db)
	if err != nil {
		log.Printf("error listing enum groups: %v", err)
		http.Error(w, "failed to load enum groups", http.StatusInternalServerError)
		return
	}

	data := categoryEditData{
		Category:   cat,
		Parents:    parents,
		Attributes: attrs,
		EnumGroups: enumGroups,
	}
	app.renderer.RenderPage(w, r, "categories/form", data)
}

func (app *App) HandleCategoryUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	var parentID *string
	if pid := r.FormValue("parent_id"); pid != "" {
		parentID = &pid
	}

	var description *string
	if desc := r.FormValue("description"); desc != "" {
		description = &desc
	}

	if err := updateCategory(app.db, id, name, parentID, description); err != nil {
		log.Printf("error updating category: %v", err)
		http.Error(w, "failed to update category", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/categories/"+id+"/edit", http.StatusSeeOther)
}

func (app *App) HandleCategoryDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := deleteCategory(app.db, id); err != nil {
		log.Printf("error deleting category: %v", err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (app *App) HandleCategoryQuickCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	// Check for existing category with same name (case-insensitive).
	cats, err := listAllCategories(app.db)
	if err == nil {
		for _, c := range cats {
			if strings.EqualFold(c.Name, name) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"id": c.ID, "name": c.Name})
				return
			}
		}
	}

	id, err := createCategory(app.db, name, nil, nil)
	if err != nil {
		log.Printf("error quick-creating category: %v", err)
		http.Error(w, "failed to create category", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id, "name": name})
}

// --- Attribute Definition Handlers ---

type attrFormData struct {
	CategoryID string
	EnumGroups []EnumGroup
}

func (app *App) HandleAttributeNewForm(w http.ResponseWriter, r *http.Request) {
	categoryID := r.PathValue("id")
	enumGroups, err := listEnumGroupsSimple(app.db)
	if err != nil {
		log.Printf("error listing enum groups: %v", err)
		http.Error(w, "failed to load enum groups", http.StatusInternalServerError)
		return
	}
	app.renderer.RenderFragment(w, "categories/_attr_form", attrFormData{
		CategoryID: categoryID,
		EnumGroups: enumGroups,
	})
}

func (app *App) HandleAttributeCreate(w http.ResponseWriter, r *http.Request) {
	categoryID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))

	attr := AttributeDefinition{
		CategoryID:  categoryID,
		Name:        r.FormValue("name"),
		DisplayName: r.FormValue("display_name"),
		DataType:    r.FormValue("data_type"),
		IsRequired:  r.FormValue("is_required") == "on",
		SortOrder:   sortOrder,
	}

	if u := r.FormValue("unit"); u != "" {
		attr.Unit = &u
	}
	if eg := r.FormValue("enum_group_id"); eg != "" {
		attr.EnumGroupID = &eg
	}

	if attr.Name == "" || attr.DisplayName == "" || attr.DataType == "" {
		http.Error(w, "name, display_name, and data_type are required", http.StatusBadRequest)
		return
	}

	id, err := createAttribute(app.db, attr)
	if err != nil {
		log.Printf("error creating attribute: %v", err)
		http.Error(w, "failed to create attribute", http.StatusInternalServerError)
		return
	}

	attr.ID = id
	// Resolve enum group name if set
	if attr.EnumGroupID != nil {
		group, err := getEnumGroup(app.db, *attr.EnumGroupID)
		if err == nil {
			attr.EnumGroupName = &group.Name
		}
	}

	app.renderer.RenderFragment(w, "categories/_attr_row", attr)
}

func (app *App) HandleAttributeUpdate(w http.ResponseWriter, r *http.Request) {
	// Placeholder for M1 — inline editing can be added later
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (app *App) HandleAttributeDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := deleteAttribute(app.db, id); err != nil {
		log.Printf("error deleting attribute: %v", err)
		http.Error(w, "failed to delete attribute", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
