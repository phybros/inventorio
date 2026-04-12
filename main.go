package main

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

//go:embed static
var staticFS embed.FS

type App struct {
	db       *sql.DB
	renderer *Renderer
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://inv:inv@localhost:5432/inventory?sslmode=disable"
	}

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("connected to database")

	if err := runMigrations(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	app := &App{
		db:       db,
		renderer: NewRenderer(),
	}

	mux := http.NewServeMux()

	// Static files
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("failed to create static sub-FS: %v", err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// Dashboard
	mux.HandleFunc("GET /{$}", app.HandleIndex)

	// Categories
	mux.HandleFunc("GET /admin/categories", app.HandleCategoryList)
	mux.HandleFunc("POST /admin/categories", app.HandleCategoryCreate)
	mux.HandleFunc("GET /admin/categories/{id}/edit", app.HandleCategoryEdit)
	mux.HandleFunc("PUT /admin/categories/{id}", app.HandleCategoryUpdate)
	mux.HandleFunc("DELETE /admin/categories/{id}", app.HandleCategoryDelete)

	// Attribute definitions
	mux.HandleFunc("POST /admin/categories/{id}/attributes", app.HandleAttributeCreate)
	mux.HandleFunc("GET /admin/categories/{id}/attributes/new", app.HandleAttributeNewForm)
	mux.HandleFunc("PUT /admin/attributes/{id}", app.HandleAttributeUpdate)
	mux.HandleFunc("DELETE /admin/attributes/{id}", app.HandleAttributeDelete)

	// Enum groups
	mux.HandleFunc("GET /admin/enums", app.HandleEnumList)
	mux.HandleFunc("POST /admin/enums", app.HandleEnumGroupCreate)
	mux.HandleFunc("DELETE /admin/enums/{id}", app.HandleEnumGroupDelete)
	mux.HandleFunc("POST /admin/enums/{id}/values", app.HandleEnumValueCreate)
	mux.HandleFunc("DELETE /admin/enums/values/{id}", app.HandleEnumValueDelete)

	// Components
	mux.HandleFunc("GET /components", app.HandleComponentList)
	mux.HandleFunc("GET /components/new", app.HandleComponentNew)
	mux.HandleFunc("GET /components/attr-fields", app.HandleComponentAttrFields)
	mux.HandleFunc("POST /components", app.HandleComponentCreate)
	mux.HandleFunc("GET /components/merge", app.HandleMergeList)
	mux.HandleFunc("GET /components/merge/preview", app.HandleMergePreview)
	mux.HandleFunc("POST /components/merge", app.HandleMergeCommit)
	mux.HandleFunc("GET /components/{id}", app.HandleComponentDetail)
	mux.HandleFunc("GET /components/{id}/edit", app.HandleComponentEdit)
	mux.HandleFunc("PUT /components/{id}", app.HandleComponentUpdate)
	mux.HandleFunc("DELETE /components/{id}", app.HandleComponentDelete)
	mux.HandleFunc("PATCH /components/{id}/quantity", app.HandleComponentQuantity)

	// Storage locations
	mux.HandleFunc("GET /admin/locations", app.HandleLocationList)
	mux.HandleFunc("POST /admin/locations", app.HandleLocationCreate)
	mux.HandleFunc("PUT /admin/locations/{id}", app.HandleLocationUpdate)
	mux.HandleFunc("DELETE /admin/locations/{id}", app.HandleLocationDelete)
	mux.HandleFunc("GET /admin/locations/{id}/label", app.HandleLocationLabel)

	// Audit log
	mux.HandleFunc("GET /admin/audit", app.HandleAuditLog)

	// Quick-create API (JSON, used by comboboxes) — component endpoint is in projects section above
	mux.HandleFunc("POST /api/categories/quick-create", app.HandleCategoryQuickCreate)
	mux.HandleFunc("POST /api/locations/quick-create", app.HandleLocationQuickCreate)
	mux.HandleFunc("POST /api/enums/quick-create", app.HandleEnumGroupQuickCreate)
	mux.HandleFunc("POST /api/enums/{id}/values/quick-create", app.HandleEnumValueQuickCreate)

	// Projects
	mux.HandleFunc("GET /projects", app.HandleProjectList)
	mux.HandleFunc("GET /projects/new", app.HandleProjectNew)
	mux.HandleFunc("POST /projects", app.HandleProjectCreate)
	mux.HandleFunc("GET /projects/{id}", app.HandleProjectDetail)
	mux.HandleFunc("GET /projects/{id}/edit", app.HandleProjectEdit)
	mux.HandleFunc("PUT /projects/{id}", app.HandleProjectUpdate)
	mux.HandleFunc("DELETE /projects/{id}", app.HandleProjectDelete)
	mux.HandleFunc("POST /projects/{id}/duplicate", app.HandleProjectDuplicate)
	mux.HandleFunc("POST /projects/{id}/bom", app.HandleBOMItemAdd)
	mux.HandleFunc("PUT /projects/bom/{itemId}", app.HandleBOMItemUpdate)
	mux.HandleFunc("DELETE /projects/bom/{itemId}", app.HandleBOMItemDelete)
	mux.HandleFunc("POST /projects/{id}/build", app.HandleProjectBuild)

	// Quick-create API (JSON, used by combobox)
	mux.HandleFunc("POST /api/components/quick-create", app.HandleComponentQuickCreate)

	// Import
	mux.HandleFunc("GET /import", app.HandleImportPage)
	mux.HandleFunc("POST /import/preview", app.HandleImportPreview)
	mux.HandleFunc("POST /import/commit", app.HandleImportCommit)

	handler := methodOverride(mux)

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// methodOverride allows HTML forms to use PUT/DELETE via a _method field.
func methodOverride(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if m := r.FormValue("_method"); m != "" {
				r.Method = m
			}
		}
		next.ServeHTTP(w, r)
	})
}

func runMigrations(db *sql.DB) error {
	source, err := iofs.New(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "inventory", driver)
	if err != nil {
		return fmt.Errorf("creating migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}

	version, dirty, _ := m.Version()
	log.Printf("database at migration version %d (dirty: %v)", version, dirty)
	return nil
}
