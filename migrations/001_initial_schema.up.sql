CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- Enum groups (reusable picklists)
CREATE TABLE enum_groups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE enum_values (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enum_group_id UUID NOT NULL REFERENCES enum_groups(id) ON DELETE CASCADE,
    value         TEXT NOT NULL,
    display_name  TEXT NOT NULL,
    sort_order    INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(enum_group_id, value)
);

-- Categories
CREATE TABLE categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    parent_id   UUID REFERENCES categories(id),
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Attribute definitions (per-category schema)
CREATE TABLE attribute_definitions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id   UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    display_name  TEXT NOT NULL,
    data_type     TEXT NOT NULL CHECK (data_type IN ('numeric', 'text', 'enum', 'boolean')),
    unit          TEXT,
    enum_group_id UUID REFERENCES enum_groups(id),
    is_required   BOOLEAN NOT NULL DEFAULT false,
    sort_order    INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(category_id, name)
);

-- Components
CREATE TABLE components (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id   UUID NOT NULL REFERENCES categories(id),
    mpn           TEXT,
    manufacturer  TEXT,
    description   TEXT,
    quantity      INT NOT NULL DEFAULT 0,
    min_quantity  INT NOT NULL DEFAULT 0,
    location      TEXT,
    datasheet_url TEXT,
    notes         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Component attribute values (EAV)
CREATE TABLE component_attributes (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    component_id             UUID NOT NULL REFERENCES components(id) ON DELETE CASCADE,
    attribute_definition_id  UUID NOT NULL REFERENCES attribute_definitions(id),
    value_numeric            NUMERIC,
    value_text               TEXT,
    value_enum               UUID REFERENCES enum_values(id),
    value_boolean            BOOLEAN,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(component_id, attribute_definition_id)
);

-- Storage locations
CREATE TABLE storage_locations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    parent_id   UUID REFERENCES storage_locations(id),
    description TEXT,
    barcode     TEXT UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Audit log
CREATE TABLE audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    table_name  TEXT NOT NULL,
    record_id   UUID NOT NULL,
    action      TEXT NOT NULL,
    old_values  JSONB,
    new_values  JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes
CREATE INDEX idx_comp_attr_numeric
    ON component_attributes (attribute_definition_id, value_numeric)
    WHERE value_numeric IS NOT NULL;

CREATE INDEX idx_comp_attr_enum
    ON component_attributes (attribute_definition_id, value_enum)
    WHERE value_enum IS NOT NULL;

CREATE INDEX idx_components_mpn
    ON components USING gin (mpn gin_trgm_ops);

CREATE INDEX idx_components_category
    ON components (category_id);

CREATE INDEX idx_components_low_stock
    ON components (category_id)
    WHERE quantity <= min_quantity AND min_quantity > 0;
