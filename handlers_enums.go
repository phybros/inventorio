package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func (app *App) HandleEnumList(w http.ResponseWriter, r *http.Request) {
	groups, err := listEnumGroupsWithValues(app.db)
	if err != nil {
		log.Printf("error listing enum groups: %v", err)
		http.Error(w, "failed to load enum groups", http.StatusInternalServerError)
		return
	}
	app.renderer.RenderPage(w, r, "enums/list", groups)
}

func (app *App) HandleEnumGroupCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	id, err := createEnumGroup(app.db, name)
	if err != nil {
		log.Printf("error creating enum group: %v", err)
		http.Error(w, "failed to create enum group", http.StatusInternalServerError)
		return
	}

	group, err := getEnumGroup(app.db, id)
	if err != nil {
		log.Printf("error fetching new enum group: %v", err)
		http.Error(w, "failed to load enum group", http.StatusInternalServerError)
		return
	}

	app.renderer.RenderFragment(w, "enums/_group", group)
}

func (app *App) HandleEnumGroupDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := deleteEnumGroup(app.db, id); err != nil {
		log.Printf("error deleting enum group: %v", err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (app *App) HandleEnumValueCreate(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	value := r.FormValue("value")
	displayName := r.FormValue("display_name")
	if value == "" || displayName == "" {
		http.Error(w, "value and display_name are required", http.StatusBadRequest)
		return
	}

	id, err := createEnumValue(app.db, groupID, value, displayName)
	if err != nil {
		log.Printf("error creating enum value: %v", err)
		http.Error(w, "failed to create enum value", http.StatusInternalServerError)
		return
	}

	v := EnumValue{
		ID:          id,
		EnumGroupID: groupID,
		Value:       value,
		DisplayName: displayName,
	}
	app.renderer.RenderFragment(w, "enums/_value_row", v)
}

func (app *App) HandleEnumGroupQuickCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	// Check for existing group with same name.
	groups, err := listEnumGroupsSimple(app.db)
	if err == nil {
		for _, g := range groups {
			if strings.EqualFold(g.Name, name) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"id": g.ID, "name": g.Name})
				return
			}
		}
	}

	id, err := createEnumGroup(app.db, name)
	if err != nil {
		log.Printf("error quick-creating enum group: %v", err)
		http.Error(w, "failed to create enum group", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id, "name": name})
}

func (app *App) HandleEnumValueQuickCreate(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	// Check for existing value with same name in this group.
	vals, err := getEnumValuesForGroup(app.db, groupID)
	if err == nil {
		for _, v := range vals {
			if strings.EqualFold(v.DisplayName, name) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"id": v.ID, "name": v.DisplayName})
				return
			}
		}
	}

	id, err := createEnumValue(app.db, groupID, name, name)
	if err != nil {
		log.Printf("error quick-creating enum value: %v", err)
		http.Error(w, "failed to create enum value", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id, "name": name})
}

func (app *App) HandleEnumValueDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := deleteEnumValue(app.db, id); err != nil {
		log.Printf("error deleting enum value: %v", err)
		http.Error(w, "failed to delete enum value", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
