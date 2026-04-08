package main

import (
	"log"
	"net/http"
	"strconv"
)

type auditLogData struct {
	Entries    []AuditLogEntry
	Page       int
	TotalPages int
	Total      int
	HasPrev    bool
	HasNext    bool
}

const auditPageSize = 50

func (app *App) HandleAuditLog(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	offset := (page - 1) * auditPageSize

	entries, total, err := listAuditLog(app.db, auditPageSize, offset)
	if err != nil {
		log.Printf("error listing audit log: %v", err)
		http.Error(w, "failed to load audit log", http.StatusInternalServerError)
		return
	}

	totalPages := (total + auditPageSize - 1) / auditPageSize
	if totalPages < 1 {
		totalPages = 1
	}

	app.renderer.RenderPage(w, "audit/list", auditLogData{
		Entries:    entries,
		Page:       page,
		TotalPages: totalPages,
		Total:      total,
		HasPrev:    page > 1,
		HasNext:    page < totalPages,
	})
}
