package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// --- Project list ---

type projectListData struct {
	Projects []ProjectListItem
	Status   string
}

func (app *App) HandleProjectList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	projects, err := listProjects(app.db, status)
	if err != nil {
		log.Printf("error listing projects: %v", err)
		http.Error(w, "failed to load projects", http.StatusInternalServerError)
		return
	}
	app.renderer.RenderPage(w, r, "projects/list", projectListData{
		Projects: projects,
		Status:   status,
	})
}

// --- Project form (new / edit) ---

type projectFormData struct {
	Project *Project
	IsNew   bool
	Errors  map[string]string
}

func (app *App) HandleProjectNew(w http.ResponseWriter, r *http.Request) {
	app.renderer.RenderPage(w, r, "projects/form", projectFormData{IsNew: true})
}

func (app *App) HandleProjectCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		app.renderer.RenderPage(w, r, "projects/form", projectFormData{
			IsNew:  true,
			Errors: map[string]string{"name": "Name is required"},
			Project: &Project{
				Status:      r.FormValue("status"),
				Description: nilStr(r.FormValue("description")),
			},
		})
		return
	}

	status := r.FormValue("status")
	if status == "" {
		status = "active"
	}
	desc := nilStr(r.FormValue("description"))

	id, err := createProject(app.db, name, desc, status)
	if err != nil {
		log.Printf("error creating project: %v", err)
		http.Error(w, "failed to create project", http.StatusInternalServerError)
		return
	}
	insertAuditLog(r, app.db, "projects", id, "insert", nil, map[string]string{"name": name, "status": status})
	http.Redirect(w, r, "/projects/"+id, http.StatusSeeOther)
}

func (app *App) HandleProjectEdit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	project, err := getProject(app.db, id)
	if err != nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	app.renderer.RenderPage(w, r, "projects/form", projectFormData{Project: project, IsNew: false})
}

func (app *App) HandleProjectUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		project, _ := getProject(app.db, id)
		app.renderer.RenderPage(w, r, "projects/form", projectFormData{
			Project: project,
			IsNew:   false,
			Errors:  map[string]string{"name": "Name is required"},
		})
		return
	}

	status := r.FormValue("status")
	if status == "" {
		status = "active"
	}
	desc := nilStr(r.FormValue("description"))

	if err := updateProject(app.db, id, name, desc, status); err != nil {
		log.Printf("error updating project: %v", err)
		http.Error(w, "failed to update project", http.StatusInternalServerError)
		return
	}
	insertAuditLog(r, app.db, "projects", id, "update", nil, map[string]string{"name": name, "status": status})
	http.Redirect(w, r, "/projects/"+id, http.StatusSeeOther)
}

func (app *App) HandleProjectDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := deleteProject(app.db, id); err != nil {
		log.Printf("error deleting project: %v", err)
		http.Error(w, "failed to delete project", http.StatusInternalServerError)
		return
	}
	insertAuditLog(r, app.db, "projects", id, "delete", nil, nil)
	w.Header().Set("HX-Redirect", "/projects")
	w.WriteHeader(http.StatusOK)
}

func (app *App) HandleProjectDuplicate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	src, err := getProject(app.db, id)
	if err != nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	copyName := src.Name + " (copy)"
	newID, err := createProject(app.db, copyName, src.Description, src.Status)
	if err != nil {
		log.Printf("error duplicating project: %v", err)
		http.Error(w, "failed to duplicate project", http.StatusInternalServerError)
		return
	}

	items, err := listBOMItems(app.db, id)
	if err == nil {
		for _, item := range items {
			if _, err := createBOMItem(app.db, newID, item.ComponentID, item.Quantity, item.Reference, item.Notes); err != nil {
				log.Printf("error copying BOM item: %v", err)
			}
		}
	}
	insertAuditLog(r, app.db, "projects", newID, "insert", nil, map[string]string{"name": copyName, "duplicated_from": id})
	http.Redirect(w, r, "/projects/"+newID, http.StatusSeeOther)
}

// --- Project detail ---

type projectDetailData struct {
	Project       *Project
	BOMItems      []BOMItem
	Builds        []ProjectBuild
	Buildable     bool
	MaxBuildable  int
	ShortageCount int
	Components    []ComponentListItem // for the add-item combobox
}

func (app *App) HandleProjectDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	project, err := getProject(app.db, id)
	if err != nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	items, err := listBOMItems(app.db, id)
	if err != nil {
		log.Printf("error listing BOM items: %v", err)
		http.Error(w, "failed to load BOM", http.StatusInternalServerError)
		return
	}

	builds, err := listProjectBuilds(app.db, id)
	if err != nil {
		log.Printf("error listing builds: %v", err)
		http.Error(w, "failed to load builds", http.StatusInternalServerError)
		return
	}

	maxBuildable, err := getMaxBuildable(app.db, id)
	if err != nil {
		log.Printf("error calculating max buildable: %v", err)
		maxBuildable = 0
	}

	components, err := listAllComponents(app.db)
	if err != nil {
		log.Printf("error listing components: %v", err)
		http.Error(w, "failed to load components", http.StatusInternalServerError)
		return
	}

	buildable := true
	shortageCount := 0
	for _, item := range items {
		if !item.Sufficient {
			buildable = false
			shortageCount++
		}
	}
	// An empty BOM is not buildable (nothing to build)
	if len(items) == 0 {
		buildable = false
	}

	app.renderer.RenderPage(w, r, "projects/detail", projectDetailData{
		Project:       project,
		BOMItems:      items,
		Builds:        builds,
		Buildable:     buildable,
		MaxBuildable:  maxBuildable,
		ShortageCount: shortageCount,
		Components:    components,
	})
}

// --- BOM item management ---

type bomTableData struct {
	ProjectID     string
	BOMItems      []BOMItem
	Buildable     bool
	MaxBuildable  int
	ShortageCount int
	Components    []ComponentListItem
}

func (app *App) renderBOMFragment(w http.ResponseWriter, projectID string) {
	items, err := listBOMItems(app.db, projectID)
	if err != nil {
		log.Printf("error listing BOM items: %v", err)
		http.Error(w, "failed to load BOM", http.StatusInternalServerError)
		return
	}
	components, err := listAllComponents(app.db)
	if err != nil {
		log.Printf("error listing components: %v", err)
		http.Error(w, "failed to load components", http.StatusInternalServerError)
		return
	}
	maxBuildable, _ := getMaxBuildable(app.db, projectID)

	buildable := len(items) > 0
	shortageCount := 0
	for _, item := range items {
		if !item.Sufficient {
			buildable = false
			shortageCount++
		}
	}

	app.renderer.RenderFragment(w, "projects/_bom_table", bomTableData{
		ProjectID:     projectID,
		BOMItems:      items,
		Buildable:     buildable,
		MaxBuildable:  maxBuildable,
		ShortageCount: shortageCount,
		Components:    components,
	})
}

func (app *App) HandleBOMItemAdd(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	componentID := r.FormValue("component_id")
	if componentID == "" {
		http.Error(w, "component is required", http.StatusBadRequest)
		return
	}

	qty, err := strconv.Atoi(r.FormValue("quantity"))
	if err != nil || qty <= 0 {
		qty = 1
	}

	var ref, notes *string
	if v := strings.TrimSpace(r.FormValue("reference")); v != "" {
		ref = &v
	}
	if v := strings.TrimSpace(r.FormValue("notes")); v != "" {
		notes = &v
	}

	_, err = createBOMItem(app.db, projectID, componentID, qty, ref, notes)
	if err != nil {
		// Likely a duplicate — return a user-friendly error
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			http.Error(w, "This component is already in the BOM", http.StatusConflict)
			return
		}
		log.Printf("error creating BOM item: %v", err)
		http.Error(w, "failed to add BOM item", http.StatusInternalServerError)
		return
	}

	app.renderBOMFragment(w, projectID)
}

func (app *App) HandleBOMItemUpdate(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	qty, err := strconv.Atoi(r.FormValue("quantity"))
	if err != nil || qty <= 0 {
		qty = 1
	}
	var ref, notes *string
	if v := strings.TrimSpace(r.FormValue("reference")); v != "" {
		ref = &v
	}
	if v := strings.TrimSpace(r.FormValue("notes")); v != "" {
		notes = &v
	}

	item, err := getBOMItem(app.db, itemID)
	if err != nil {
		http.Error(w, "BOM item not found", http.StatusNotFound)
		return
	}

	if err := updateBOMItem(app.db, itemID, qty, ref, notes); err != nil {
		log.Printf("error updating BOM item: %v", err)
		http.Error(w, "failed to update BOM item", http.StatusInternalServerError)
		return
	}

	app.renderBOMFragment(w, item.ProjectID)
}

func (app *App) HandleBOMItemDelete(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	projectID, err := deleteBOMItem(app.db, itemID)
	if err != nil {
		log.Printf("error deleting BOM item: %v", err)
		http.Error(w, "failed to delete BOM item", http.StatusInternalServerError)
		return
	}
	app.renderBOMFragment(w, projectID)
}

// --- Build ---

type buildResultData struct {
	Success      bool
	Build        *ProjectBuild
	ShortItems   []BOMItem
	Multiplier   int
}

func (app *App) HandleProjectBuild(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	multiplier, err := strconv.Atoi(r.FormValue("multiplier"))
	if err != nil || multiplier <= 0 {
		multiplier = 1
	}
	var notes *string
	if v := strings.TrimSpace(r.FormValue("notes")); v != "" {
		notes = &v
	}

	build, shortItems, err := executeProjectBuild(app.db, projectID, multiplier, notes)
	if err != nil && build == nil {
		if len(shortItems) > 0 {
			app.renderer.RenderFragment(w, "projects/_build_result", buildResultData{
				Success:    false,
				ShortItems: shortItems,
				Multiplier: multiplier,
			})
			return
		}
		log.Printf("error executing build: %v", err)
		http.Error(w, "failed to execute build", http.StatusInternalServerError)
		return
	}

	insertAuditLog(r, app.db, "project_builds", build.ID, "insert", nil,
		map[string]any{"project_id": projectID, "multiplier": multiplier})

	app.renderer.RenderFragment(w, "projects/_build_result", buildResultData{
		Success:    true,
		Build:      build,
		Multiplier: multiplier,
	})
}

// --- Component quick-create (for BOM combobox) ---

func (app *App) HandleComponentQuickCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	mpn := strings.TrimSpace(r.FormValue("name"))
	if mpn == "" {
		http.Error(w, "MPN is required", http.StatusBadRequest)
		return
	}

	// Check if a component with this MPN already exists
	existing, err := findComponentByMPN(app.db, mpn)
	if err == nil && existing != nil {
		w.Header().Set("Content-Type", "application/json")
		label := mpn
		if existing.Manufacturer != nil {
			label = mpn + " — " + *existing.Manufacturer
		}
		json.NewEncoder(w).Encode(map[string]string{"id": existing.ID, "name": label})
		return
	}

	// Need a category — use the first available or create a placeholder
	cats, err := listAllCategories(app.db)
	if err != nil || len(cats) == 0 {
		http.Error(w, "no categories available to create placeholder component", http.StatusUnprocessableEntity)
		return
	}
	// Use the first root-level category as the placeholder category
	categoryID := cats[0].ID
	for _, c := range cats {
		if c.ParentID == nil {
			categoryID = c.ID
			break
		}
	}

	comp := &Component{
		CategoryID: categoryID,
		MPN:        &mpn,
		Quantity:   0,
	}
	id, err := createComponent(app.db, comp)
	if err != nil {
		log.Printf("error creating placeholder component: %v", err)
		http.Error(w, "failed to create component", http.StatusInternalServerError)
		return
	}
	insertAuditLog(r, app.db, "components", id, "insert", nil, map[string]string{"mpn": mpn, "note": "placeholder"})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "name": mpn + " (placeholder)"})
}

// nilStr returns nil if the string is empty, otherwise a pointer to it.
func nilStr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
