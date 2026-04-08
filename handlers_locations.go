package main

import (
	"log"
	"net/http"
)

type locationListData struct {
	Locations []StorageLocation
}

func (app *App) HandleLocationList(w http.ResponseWriter, r *http.Request) {
	locations, err := listStorageLocations(app.db)
	if err != nil {
		log.Printf("error listing locations: %v", err)
		http.Error(w, "failed to load locations", http.StatusInternalServerError)
		return
	}
	app.renderer.RenderPage(w, "locations/list", locationListData{Locations: locations})
}

func (app *App) HandleLocationCreate(w http.ResponseWriter, r *http.Request) {
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
	if v := r.FormValue("parent_id"); v != "" {
		parentID = &v
	}
	var description *string
	if v := r.FormValue("description"); v != "" {
		description = &v
	}

	if _, err := createStorageLocation(app.db, name, parentID, description); err != nil {
		log.Printf("error creating location: %v", err)
		http.Error(w, "failed to create location", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/locations", http.StatusSeeOther)
}

func (app *App) HandleLocationUpdate(w http.ResponseWriter, r *http.Request) {
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
	if v := r.FormValue("parent_id"); v != "" {
		parentID = &v
	}
	var description *string
	if v := r.FormValue("description"); v != "" {
		description = &v
	}

	if err := updateStorageLocation(app.db, id, name, parentID, description); err != nil {
		log.Printf("error updating location: %v", err)
		http.Error(w, "failed to update location", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/locations", http.StatusSeeOther)
}

func (app *App) HandleLocationDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := deleteStorageLocation(app.db, id); err != nil {
		log.Printf("error deleting location: %v", err)
		http.Error(w, "failed to delete location", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (app *App) HandleLocationLabel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	loc, err := getStorageLocation(app.db, id)
	if err != nil {
		log.Printf("error fetching location: %v", err)
		http.Error(w, "location not found", http.StatusNotFound)
		return
	}
	app.renderer.RenderFragment(w, "locations/_label", loc)
}
