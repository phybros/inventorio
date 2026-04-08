# Electronics Component Inventory System — MVP Spec

## Overview

A self-hosted web application for managing a personal electronics component inventory. The system uses a category-driven data model where categories define the attribute schema for their components, enabling parametric search and structured data entry across diverse component types (passives, logic ICs, connectors, etc.).

---

## Tech Stack

| Layer       | Choice                        | Rationale                                                |
|-------------|-------------------------------|----------------------------------------------------------|
| Backend     | Go (stdlib `net/http` + `pgx`) | Single binary, low ops burden, no framework churn        |
| Database    | PostgreSQL 16                 | JSONB, GIN indexes, full-text search, range queries      |
| Frontend    | HTMX + Go `html/template`    | CRUD-native, no JS build pipeline, progressive enhancement |
| CSS         | Tailwind (CDN standalone)     | Utility classes, no build step needed                    |
| Deployment  | Docker Compose                | App container + Postgres, single `docker-compose up`     |
| Reverse proxy | Caddy                       | Auto-TLS, simple config, zero-downtime reload            |

---

## Data Model

### Core Tables

```
categories
├── id              UUID PK
├── name            TEXT NOT NULL UNIQUE
├── parent_id       UUID FK → categories (nullable, one level max)
├── description     TEXT
├── created_at      TIMESTAMPTZ
└── updated_at      TIMESTAMPTZ

attribute_definitions
├── id              UUID PK
├── category_id     UUID FK → categories
├── name            TEXT NOT NULL          -- e.g. "resistance", "logic_family"
├── display_name    TEXT NOT NULL          -- e.g. "Resistance", "Logic Family"
├── data_type       TEXT NOT NULL          -- 'numeric', 'text', 'enum', 'boolean'
├── unit            TEXT                   -- e.g. "Ω", "µF", "V", nullable
├── enum_group_id   UUID FK → enum_groups -- nullable, only for data_type='enum'
├── is_required     BOOLEAN DEFAULT false
├── sort_order      INT DEFAULT 0         -- display ordering in forms/tables
├── created_at      TIMESTAMPTZ
└── updated_at      TIMESTAMPTZ

UNIQUE(category_id, name)

enum_groups
├── id              UUID PK
├── name            TEXT NOT NULL UNIQUE   -- e.g. "package_types", "logic_families"
└── created_at      TIMESTAMPTZ

enum_values
├── id              UUID PK
├── enum_group_id   UUID FK → enum_groups
├── value           TEXT NOT NULL
├── display_name    TEXT NOT NULL
├── sort_order      INT DEFAULT 0
└── created_at      TIMESTAMPTZ

UNIQUE(enum_group_id, value)

components
├── id              UUID PK
├── category_id     UUID FK → categories
├── mpn             TEXT                   -- manufacturer part number
├── manufacturer    TEXT
├── description     TEXT
├── quantity         INT NOT NULL DEFAULT 0
├── min_quantity     INT DEFAULT 0         -- reorder threshold
├── location        TEXT                   -- e.g. "Drawer A3, Bin 2"
├── datasheet_url   TEXT
├── notes           TEXT
├── created_at      TIMESTAMPTZ
└── updated_at      TIMESTAMPTZ

component_attributes
├── id                    UUID PK
├── component_id          UUID FK → components ON DELETE CASCADE
├── attribute_definition_id UUID FK → attribute_definitions
├── value_numeric         NUMERIC           -- used when data_type='numeric'
├── value_text            TEXT              -- used when data_type='text'
├── value_enum            UUID FK → enum_values -- used when data_type='enum'
├── value_boolean         BOOLEAN           -- used when data_type='boolean'
└── created_at            TIMESTAMPTZ

UNIQUE(component_id, attribute_definition_id)
```

### Supporting Tables

```
storage_locations
├── id              UUID PK
├── name            TEXT NOT NULL          -- e.g. "Drawer A3"
├── parent_id       UUID FK → storage_locations (nullable)
├── description     TEXT
├── barcode         TEXT UNIQUE            -- generated QR/barcode ID
└── created_at      TIMESTAMPTZ

audit_log
├── id              UUID PK
├── table_name      TEXT NOT NULL
├── record_id       UUID NOT NULL
├── action          TEXT NOT NULL          -- 'insert', 'update', 'delete'
├── old_values      JSONB
├── new_values      JSONB
├── created_at      TIMESTAMPTZ DEFAULT now()
```

### Key Indexes

```sql
-- Parametric search: find components by numeric attribute ranges
CREATE INDEX idx_comp_attr_numeric
  ON component_attributes (attribute_definition_id, value_numeric)
  WHERE value_numeric IS NOT NULL;

-- Enum attribute lookups
CREATE INDEX idx_comp_attr_enum
  ON component_attributes (attribute_definition_id, value_enum)
  WHERE value_enum IS NOT NULL;

-- Part number search
CREATE INDEX idx_components_mpn ON components USING gin (mpn gin_trgm_ops);

-- Category lookups
CREATE INDEX idx_components_category ON components (category_id);

-- Low stock alerts
CREATE INDEX idx_components_low_stock
  ON components (category_id)
  WHERE quantity <= min_quantity AND min_quantity > 0;
```

---

## Category Inheritance

Categories support one level of parent → child inheritance.

- A **parent category** (e.g. `Logic ICs`) defines common attributes: `supply_voltage`, `package`, `propagation_delay`.
- A **child category** (e.g. `Shift Registers`) inherits all parent attributes and adds its own: `num_bits`, `serial_parallel`.
- When rendering a form for a child category, the system merges parent + child attribute definitions, ordered by `sort_order`.
- Components always belong to a **leaf category** (child if one exists, otherwise a standalone parent).

---

## Seed Data (MVP)

### Enum Groups

| Group              | Values                                                              |
|--------------------|---------------------------------------------------------------------|
| `package_types`    | 0402, 0603, 0805, 1206, DIP-8, DIP-14, DIP-16, SOIC-8, SOIC-14, SOIC-16, SSOP, TSSOP, QFP, SOT-23, TO-92, TO-220 |
| `logic_families`   | HC, HCT, LS, ALS, AC, ACT, LVC, LV, F, S, CMOS 4000              |
| `tolerance_bands`  | 1%, 2%, 5%, 10%, 20%                                                |
| `capacitor_types`  | Ceramic (MLCC), Electrolytic, Tantalum, Film, Mica                  |
| `dielectric_codes` | C0G/NP0, X5R, X7R, Y5V, Z5U                                       |
| `gate_functions`   | AND, NAND, OR, NOR, XOR, XNOR, NOT, Buffer                        |

### Category Tree

```
Resistors
  └── attrs: resistance (numeric, Ω), tolerance (enum), power_rating (numeric, W), package (enum)

Capacitors
  └── attrs: capacitance (numeric, F), voltage_rating (numeric, V), capacitor_type (enum),
             dielectric (enum), tolerance (enum), package (enum)

Inductors
  └── attrs: inductance (numeric, H), current_rating (numeric, A), dcr (numeric, Ω), package (enum)

Diodes
  └── attrs: forward_voltage (numeric, V), max_current (numeric, A), reverse_voltage (numeric, V),
             type (enum: Rectifier/Schottky/Zener/LED), package (enum)

Logic ICs                              ← parent
  ├── common attrs: logic_family (enum), supply_voltage_min (numeric, V),
  │                 supply_voltage_max (numeric, V), propagation_delay (numeric, ns), package (enum)
  ├── Gates                            ← child
  │     └── attrs: gate_function (enum), num_gates (numeric)
  ├── Flip-Flops                       ← child
  │     └── attrs: type (enum: D/JK/SR/T), num_elements (numeric), edge (enum: rising/falling)
  ├── Shift Registers                  ← child
  │     └── attrs: num_bits (numeric), direction (enum: serial-in-parallel-out / PISO / bidirectional)
  ├── Buffers & Drivers                ← child
  │     └── attrs: num_channels (numeric), tri_state (boolean), direction (enum: uni/bi)
  └── Counters                         ← child
        └── attrs: num_bits (numeric), type (enum: binary/decade/up-down)

Connectors
  └── attrs: num_pins (numeric), pitch (numeric, mm), type (enum: header/socket/JST/barrel/USB/...),
             gender (enum: male/female), mounting (enum: through-hole/SMD)

Transistors
  └── attrs: type (enum: NPN/PNP/N-MOSFET/P-MOSFET/JFET), max_current (numeric, A),
             max_voltage (numeric, V), package (enum)

Voltage Regulators
  └── attrs: output_voltage (numeric, V), output_current (numeric, A),
             type (enum: linear/switching/LDO), dropout_voltage (numeric, V), package (enum)

Crystals & Oscillators
  └── attrs: frequency (numeric, Hz), load_capacitance (numeric, F), tolerance (numeric, ppm), package (enum)

Misc / Unsorted
  └── attrs: (none — freeform notes only, catch-all for uncategorized parts)
```

---

## Pages & Routes

### 1. Dashboard — `GET /`

- Total component count, total unique parts
- Low stock alerts (quantity ≤ min_quantity)
- Recently added / modified components
- Quick search bar (MPN / description, global)

### 2. Component List — `GET /components?category=:id&...`

- Category sidebar/dropdown to filter
- When a category is selected, show parametric filter controls generated from its attribute definitions:
  - Numeric → min/max range inputs with unit label
  - Enum → multi-select dropdown
  - Boolean → checkbox
  - Text → search input
- Results table with sortable columns: MPN, manufacturer, category-specific attributes, quantity, location
- Inline quantity adjustment (+/- buttons, HTMX PATCH)
- Pagination

### 3. Component Detail — `GET /components/:id`

- All fields and attribute values displayed
- Edit button → switches to edit mode (HTMX swap)
- Quantity adjustment with audit trail
- Datasheet link
- Audit history for this component

### 4. Component Create/Edit — `GET /components/new?category=:id` / `GET /components/:id/edit`

- Category selector (locked after creation)
- Dynamic form fields rendered from attribute_definitions for the selected category
- Client-side validation (required fields, numeric ranges) via minimal JS
- MPN auto-lookup (future: query Octopart/Mouser API — not MVP)

### 5. Category Management — `GET /admin/categories`

- CRUD for categories and their parent/child relationships
- Attribute definition editor per category:
  - Add/remove/reorder attributes
  - Set data type, unit, required flag, enum group
- Enum group management (add values to existing groups, create new groups)

### 6. Storage Locations — `GET /admin/locations`

- Tree view of storage locations
- Assign components to locations
- Generate printable QR code labels (SVG, sized for label printer)

### 7. Import/Export — `GET /admin/import`

- CSV import: upload CSV, map columns to category + attributes
- CSV export: per-category or full inventory dump

---

## API Routes (HTMX-driven, returning HTML fragments)

```
GET    /                              Dashboard
GET    /components                    List (filterable)
GET    /components/new                Create form (category selector first)
POST   /components                    Create component
GET    /components/:id                Detail view
GET    /components/:id/edit           Edit form
PUT    /components/:id                Update component
DELETE /components/:id                Delete component
PATCH  /components/:id/quantity       Inline quantity adjustment (+/- delta)

GET    /admin/categories              Category list
POST   /admin/categories              Create category
GET    /admin/categories/:id/edit     Edit category + attributes
PUT    /admin/categories/:id          Update category
DELETE /admin/categories/:id          Delete category (blocked if components exist)

GET    /admin/enums                   Enum group management
POST   /admin/enums                   Create enum group
POST   /admin/enums/:id/values        Add value to enum group

GET    /admin/locations               Storage location tree
POST   /admin/locations               Create location
GET    /admin/locations/:id/label     Generate QR label (SVG)

POST   /admin/import                  CSV import
GET    /admin/export                  CSV export (query params for filtering)

GET    /api/search?q=...              Global search (JSON for autocomplete)
```

---

## 74-Series Part Number Parser

A Go function that decomposes standard 74-series part numbers:

```
Input:  "SN74HC595N"
Output: {
  prefix:   "SN74" or "74",
  family:   "HC",
  function: "595",
  suffix:   "N" (package hint: DIP),
  suggested_category: "Shift Registers",
  suggested_attrs: {
    logic_family: "HC"
  }
}
```

Integrated into the component creation form: typing a recognized part number auto-selects the category and pre-fills known attributes. Falls back gracefully for unrecognized patterns.

---

## Non-Goals (MVP)

These are explicitly out of scope for the first version:

- Multi-user auth (single-user, run on LAN behind firewall/VPN)
- Octopart/Mouser/Digi-Key API integration for auto-populating specs
- BOM management and project kits (design validated, build later)
- Purchase order tracking
- Image uploads for components
- Mobile-native app (responsive web is sufficient)
- Full-text search across datasheets

---

## Deployment

```yaml
# docker-compose.yml (simplified)
services:
  app:
    build: .
    ports: ["8080:8080"]
    environment:
      DATABASE_URL: postgres://inv:inv@db:5432/inventory?sslmode=disable
    depends_on: [db]

  db:
    image: postgres:16-alpine
    volumes: ["pgdata:/var/lib/postgresql/data"]
    environment:
      POSTGRES_DB: inventory
      POSTGRES_USER: inv
      POSTGRES_PASSWORD: inv

volumes:
  pgdata:
```

### Backup

```bash
# Cron job: daily pg_dump to local + optional rsync to NAS
0 3 * * * docker exec inventory-db pg_dump -U inv inventory | gzip > /backups/inventory-$(date +\%F).sql.gz
```

---

## Development Milestones

### M1 — Data layer & category admin (Week 1–2)
- Database schema, migrations (golang-migrate)
- Category CRUD with attribute definition editor
- Enum group management
- Seed data loader

### M2 — Component CRUD & dynamic forms (Week 3–4)
- Component create/edit with dynamically rendered attribute forms
- Component list with basic filtering (category only)
- Component detail view
- Inline quantity adjustment

### M3 — Parametric search & dashboard (Week 5–6)
- Parametric filter UI generated from attribute definitions
- Global search (MPN, description)
- Dashboard with low stock alerts, recent activity
- Audit log (triggered via Postgres trigger or app-level)

### M4 — Import/export, locations, polish (Week 7–8)
- CSV import with column mapping UI
- CSV export
- Storage location tree + QR label generation
- 74-series part number parser
- Responsive layout pass
- Docker Compose production config with Caddy
