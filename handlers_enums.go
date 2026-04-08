package main

import (
	"log"
	"net/http"
)

func (app *App) HandleEnumList(w http.ResponseWriter, r *http.Request) {
	groups, err := listEnumGroupsWithValues(app.db)
	if err != nil {
		log.Printf("error listing enum groups: %v", err)
		http.Error(w, "failed to load enum groups", http.StatusInternalServerError)
		return
	}
	app.renderer.RenderPage(w, "enums/list", groups)
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

func (app *App) HandleEnumValueDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := deleteEnumValue(app.db, id); err != nil {
		log.Printf("error deleting enum value: %v", err)
		http.Error(w, "failed to delete enum value", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
