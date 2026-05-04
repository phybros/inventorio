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
    parent_id   UUID REFERENCES categories(id) ON DELETE RESTRICT,
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
    location_id   UUID,
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

ALTER TABLE components
    ADD CONSTRAINT components_location_id_fkey
    FOREIGN KEY (location_id) REFERENCES storage_locations(id) ON DELETE SET NULL;

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

CREATE INDEX idx_categories_parent
    ON categories (parent_id);

CREATE OR REPLACE FUNCTION prevent_category_cycles()
RETURNS trigger AS $$
BEGIN
    IF NEW.parent_id IS NULL THEN
        RETURN NEW;
    END IF;

    IF NEW.parent_id = NEW.id THEN
        RAISE EXCEPTION 'category cannot be its own parent';
    END IF;

    IF EXISTS (
        WITH RECURSIVE ancestors AS (
            SELECT id, parent_id
            FROM categories
            WHERE id = NEW.parent_id

            UNION ALL

            SELECT c.id, c.parent_id
            FROM categories c
            JOIN ancestors a ON c.id = a.parent_id
        )
        SELECT 1
        FROM ancestors
        WHERE id = NEW.id
    ) THEN
        RAISE EXCEPTION 'category parent would create a cycle';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_prevent_category_cycles
    BEFORE INSERT OR UPDATE OF parent_id ON categories
    FOR EACH ROW
    EXECUTE FUNCTION prevent_category_cycles();

-- Additional component search indexes
CREATE INDEX IF NOT EXISTS idx_components_manufacturer
    ON components USING gin (manufacturer gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_components_description
    ON components USING gin (description gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_components_notes
    ON components USING gin (notes gin_trgm_ops);


-- Projects and BOMs
CREATE TABLE projects (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT,
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('draft', 'active', 'archived')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE project_bom_items (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    component_id  UUID NOT NULL REFERENCES components(id),
    quantity      INT NOT NULL CHECK (quantity > 0),
    reference     TEXT,
    notes         TEXT,
    sort_order    INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(project_id, component_id)
);

CREATE TABLE project_builds (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    multiplier  INT NOT NULL DEFAULT 1 CHECK (multiplier > 0),
    notes       TEXT,
    built_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_bom_project ON project_bom_items (project_id);
CREATE INDEX idx_bom_component ON project_bom_items (component_id);
CREATE INDEX idx_builds_project ON project_builds (project_id);


-- Authentication
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    display_name TEXT,
    avatar_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('github', 'google', 'proxy')),
    provider_subject TEXT NOT NULL,
    provider_login TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider, provider_subject)
);

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE audit_log
ADD COLUMN actor_user_id UUID REFERENCES users(id),
ADD COLUMN actor_email TEXT;

CREATE INDEX idx_user_identities_user_id ON user_identities(user_id);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_audit_log_actor_user_id ON audit_log(actor_user_id);


-- Seed data
-- Seed data: enum groups, categories, and attribute definitions
-- Uses hardcoded UUIDs so attribute_definitions can reference enum_group IDs

-- ============================================================
-- Enum Group UUIDs
-- ============================================================
-- package_types:         a0000000-0000-0000-0000-000000000001
-- logic_families:        a0000000-0000-0000-0000-000000000002
-- tolerance_bands:       a0000000-0000-0000-0000-000000000003
-- capacitor_types:       a0000000-0000-0000-0000-000000000004
-- dielectric_codes:      a0000000-0000-0000-0000-000000000005
-- gate_functions:        a0000000-0000-0000-0000-000000000006
-- diode_types:           a0000000-0000-0000-0000-000000000007
-- flipflop_types:        a0000000-0000-0000-0000-000000000008
-- edge_types:            a0000000-0000-0000-0000-000000000009
-- shift_reg_directions:  a0000000-0000-0000-0000-00000000000a
-- buffer_directions:     a0000000-0000-0000-0000-00000000000b
-- counter_types:         a0000000-0000-0000-0000-00000000000c
-- connector_types:       a0000000-0000-0000-0000-00000000000d
-- connector_gender:      a0000000-0000-0000-0000-00000000000e
-- mounting_types:        a0000000-0000-0000-0000-00000000000f
-- transistor_types:      a0000000-0000-0000-0000-000000000010
-- regulator_types:       a0000000-0000-0000-0000-000000000011

-- ============================================================
-- Category UUIDs
-- ============================================================
-- Resistors:             b0000000-0000-0000-0000-000000000001
-- Capacitors:            b0000000-0000-0000-0000-000000000002
-- Inductors:             b0000000-0000-0000-0000-000000000003
-- Diodes:                b0000000-0000-0000-0000-000000000004
-- Logic ICs:             b0000000-0000-0000-0000-000000000005
-- Gates:                 b0000000-0000-0000-0000-000000000006
-- Flip-Flops:            b0000000-0000-0000-0000-000000000007
-- Shift Registers:       b0000000-0000-0000-0000-000000000008
-- Buffers & Drivers:     b0000000-0000-0000-0000-000000000009
-- Counters:              b0000000-0000-0000-0000-00000000000a
-- Connectors:            b0000000-0000-0000-0000-00000000000b
-- Transistors:           b0000000-0000-0000-0000-00000000000c
-- Voltage Regulators:    b0000000-0000-0000-0000-00000000000d
-- Crystals & Oscillators:b0000000-0000-0000-0000-00000000000e
-- Misc / Unsorted:       b0000000-0000-0000-0000-00000000000f

-- ============================================================
-- ENUM GROUPS
-- ============================================================

INSERT INTO enum_groups (id, name) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'package_types'),
    ('a0000000-0000-0000-0000-000000000002', 'logic_families'),
    ('a0000000-0000-0000-0000-000000000003', 'tolerance_bands'),
    ('a0000000-0000-0000-0000-000000000004', 'capacitor_types'),
    ('a0000000-0000-0000-0000-000000000005', 'dielectric_codes'),
    ('a0000000-0000-0000-0000-000000000006', 'gate_functions'),
    ('a0000000-0000-0000-0000-000000000007', 'diode_types'),
    ('a0000000-0000-0000-0000-000000000008', 'flipflop_types'),
    ('a0000000-0000-0000-0000-000000000009', 'edge_types'),
    ('a0000000-0000-0000-0000-00000000000a', 'shift_register_directions'),
    ('a0000000-0000-0000-0000-00000000000b', 'buffer_directions'),
    ('a0000000-0000-0000-0000-00000000000c', 'counter_types'),
    ('a0000000-0000-0000-0000-00000000000d', 'connector_types'),
    ('a0000000-0000-0000-0000-00000000000e', 'connector_gender'),
    ('a0000000-0000-0000-0000-00000000000f', 'mounting_types'),
    ('a0000000-0000-0000-0000-000000000010', 'transistor_types'),
    ('a0000000-0000-0000-0000-000000000011', 'regulator_types');

-- ============================================================
-- ENUM VALUES
-- ============================================================

-- package_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000001', '0402',    '0402',    0),
    ('a0000000-0000-0000-0000-000000000001', '0603',    '0603',    1),
    ('a0000000-0000-0000-0000-000000000001', '0805',    '0805',    2),
    ('a0000000-0000-0000-0000-000000000001', '1206',    '1206',    3),
    ('a0000000-0000-0000-0000-000000000001', 'DIP-8',   'DIP-8',   4),
    ('a0000000-0000-0000-0000-000000000001', 'DIP-14',  'DIP-14',  5),
    ('a0000000-0000-0000-0000-000000000001', 'DIP-16',  'DIP-16',  6),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-8',  'SOIC-8',  7),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-14', 'SOIC-14', 8),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-16', 'SOIC-16', 9),
    ('a0000000-0000-0000-0000-000000000001', 'SSOP',    'SSOP',    10),
    ('a0000000-0000-0000-0000-000000000001', 'TSSOP',   'TSSOP',   11),
    ('a0000000-0000-0000-0000-000000000001', 'QFP',     'QFP',     12),
    ('a0000000-0000-0000-0000-000000000001', 'SOT-23',  'SOT-23',  13),
    ('a0000000-0000-0000-0000-000000000001', 'TO-92',   'TO-92',   14),
    ('a0000000-0000-0000-0000-000000000001', 'TO-220',  'TO-220',  15),
    ('a0000000-0000-0000-0000-000000000001', 'DIP-20',  'DIP-20',  16),
    ('a0000000-0000-0000-0000-000000000001', 'DIP-24',  'DIP-24',  17),
    ('a0000000-0000-0000-0000-000000000001', 'DIP-28',  'DIP-28',  18),
    ('a0000000-0000-0000-0000-000000000001', 'DIP-40',  'DIP-40',  19),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-8-NARROW',   'SOIC-8 (0.154", 3.90mm)',   20),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-8-WIDE',     'SOIC-8W (0.208", 5.30mm)',  21),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-14-NARROW',  'SOIC-14 (0.154", 3.90mm)',  22),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-14-WIDE',    'SOIC-14W (0.295", 7.50mm)', 23),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-16-NARROW',  'SOIC-16 (0.154", 3.90mm)',  24),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-16-WIDE',    'SOIC-16W (0.295", 7.50mm)', 25),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-20-WIDE',    'SOIC-20W (0.295", 7.50mm)', 26),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-24-WIDE',    'SOIC-24W (0.295", 7.50mm)', 27),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-28-WIDE',    'SOIC-28W (0.295", 7.50mm)', 28),
    ('a0000000-0000-0000-0000-000000000001', 'SSOP-8',          'SSOP-8',          29),
    ('a0000000-0000-0000-0000-000000000001', 'SSOP-14',         'SSOP-14',         30),
    ('a0000000-0000-0000-0000-000000000001', 'SSOP-16',         'SSOP-16',         31),
    ('a0000000-0000-0000-0000-000000000001', 'SSOP-20',         'SSOP-20',         32),
    ('a0000000-0000-0000-0000-000000000001', 'SSOP-24',         'SSOP-24',         33),
    ('a0000000-0000-0000-0000-000000000001', 'SSOP-28',         'SSOP-28',         34),
    ('a0000000-0000-0000-0000-000000000001', 'TSSOP-8',         'TSSOP-8',         35),
    ('a0000000-0000-0000-0000-000000000001', 'TSSOP-14',        'TSSOP-14',        36),
    ('a0000000-0000-0000-0000-000000000001', 'TSSOP-16',        'TSSOP-16',        37),
    ('a0000000-0000-0000-0000-000000000001', 'TSSOP-20',        'TSSOP-20',        38),
    ('a0000000-0000-0000-0000-000000000001', 'TSSOP-24',        'TSSOP-24',        39),
    ('a0000000-0000-0000-0000-000000000001', 'TSSOP-28',        'TSSOP-28',        40),
    ('a0000000-0000-0000-0000-000000000001', 'MSOP-8',          'MSOP-8',          41),
    ('a0000000-0000-0000-0000-000000000001', 'MSOP-10',         'MSOP-10',         42),
    ('a0000000-0000-0000-0000-000000000001', 'DFN-8',           'DFN-8',           43),
    ('a0000000-0000-0000-0000-000000000001', 'QFN-16',          'QFN-16',          44),
    ('a0000000-0000-0000-0000-000000000001', 'QFN-20',          'QFN-20',          45),
    ('a0000000-0000-0000-0000-000000000001', 'QFN-24',          'QFN-24',          46),
    ('a0000000-0000-0000-0000-000000000001', 'QFN-32',          'QFN-32',          47),
    ('a0000000-0000-0000-0000-000000000001', 'QFN-48',          'QFN-48',          48),
    ('a0000000-0000-0000-0000-000000000001', 'LQFP-32',         'LQFP-32',         49),
    ('a0000000-0000-0000-0000-000000000001', 'LQFP-44',         'LQFP-44',         50),
    ('a0000000-0000-0000-0000-000000000001', 'LQFP-48',         'LQFP-48',         51),
    ('a0000000-0000-0000-0000-000000000001', 'LQFP-64',         'LQFP-64',         52),
    ('a0000000-0000-0000-0000-000000000001', 'TQFP-32',         'TQFP-32',         53),
    ('a0000000-0000-0000-0000-000000000001', 'TQFP-44',         'TQFP-44',         54),
    ('a0000000-0000-0000-0000-000000000001', 'TQFP-64',         'TQFP-64',         55),
    ('a0000000-0000-0000-0000-000000000001', 'TQFP-100',        'TQFP-100',        56);

-- logic_families
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000002', 'HC',        'HC',        0),
    ('a0000000-0000-0000-0000-000000000002', 'HCT',       'HCT',       1),
    ('a0000000-0000-0000-0000-000000000002', 'LS',        'LS',        2),
    ('a0000000-0000-0000-0000-000000000002', 'AC',        'AC',        3),
    ('a0000000-0000-0000-0000-000000000002', 'ACT',       'ACT',       4),
    ('a0000000-0000-0000-0000-000000000002', '4000',      '4000',      5);

-- tolerance_bands
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000003', '1pct',  '1%',  0),
    ('a0000000-0000-0000-0000-000000000003', '2pct',  '2%',  1),
    ('a0000000-0000-0000-0000-000000000003', '5pct',  '5%',  2),
    ('a0000000-0000-0000-0000-000000000003', '10pct', '10%', 3),
    ('a0000000-0000-0000-0000-000000000003', '20pct', '20%', 4);

-- capacitor_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000004', 'ceramic',      'Ceramic', 0),
    ('a0000000-0000-0000-0000-000000000004', 'electrolytic', 'Electrolytic',   1),
    ('a0000000-0000-0000-0000-000000000004', 'tantalum',     'Tantalum',       2),
    ('a0000000-0000-0000-0000-000000000004', 'film',         'Film',           3),
    ('a0000000-0000-0000-0000-000000000004', 'mica',         'Mica',           4);

-- dielectric_codes
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000005', 'C0G_NP0', 'C0G/NP0', 0),
    ('a0000000-0000-0000-0000-000000000005', 'X5R',     'X5R',     1),
    ('a0000000-0000-0000-0000-000000000005', 'X7R',     'X7R',     2),
    ('a0000000-0000-0000-0000-000000000005', 'Y5V',     'Y5V',     3),
    ('a0000000-0000-0000-0000-000000000005', 'Z5U',     'Z5U',     4);

-- gate_functions
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000006', 'AND',    'AND',    0),
    ('a0000000-0000-0000-0000-000000000006', 'NAND',   'NAND',   1),
    ('a0000000-0000-0000-0000-000000000006', 'OR',     'OR',     2),
    ('a0000000-0000-0000-0000-000000000006', 'NOR',    'NOR',    3),
    ('a0000000-0000-0000-0000-000000000006', 'XOR',    'XOR',    4),
    ('a0000000-0000-0000-0000-000000000006', 'XNOR',   'XNOR',   5),
    ('a0000000-0000-0000-0000-000000000006', 'NOT',    'NOT',    6),
    ('a0000000-0000-0000-0000-000000000006', 'Buffer', 'Buffer', 7);

-- diode_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000007', 'rectifier', 'Rectifier', 0),
    ('a0000000-0000-0000-0000-000000000007', 'schottky',  'Schottky',  1),
    ('a0000000-0000-0000-0000-000000000007', 'zener',     'Zener',     2),
    ('a0000000-0000-0000-0000-000000000007', 'led',       'LED',       3);

-- flipflop_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000008', 'D',  'D',  0),
    ('a0000000-0000-0000-0000-000000000008', 'JK', 'JK', 1),
    ('a0000000-0000-0000-0000-000000000008', 'SR', 'SR', 2);

-- edge_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000009', 'rising',  'Rising',  0),
    ('a0000000-0000-0000-0000-000000000009', 'falling', 'Falling', 1);

-- shift_register_directions
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-00000000000a', 'SIPO',          'Serial-In Parallel-Out', 0),
    ('a0000000-0000-0000-0000-00000000000a', 'PISO',          'Parallel-In Serial-Out', 1),
    ('a0000000-0000-0000-0000-00000000000a', 'bidirectional', 'Bidirectional',          2);

-- buffer_directions
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-00000000000b', 'uni', 'Unidirectional', 0),
    ('a0000000-0000-0000-0000-00000000000b', 'bi',  'Bidirectional',  1);

-- counter_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-00000000000c', 'binary',  'Binary',  0),
    ('a0000000-0000-0000-0000-00000000000c', 'decade',  'Decade',  1),
    ('a0000000-0000-0000-0000-00000000000c', 'up_down', 'Up/Down', 2);

-- connector_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-00000000000d', 'header', 'Header', 0),
    ('a0000000-0000-0000-0000-00000000000d', 'socket', 'Socket', 1),
    ('a0000000-0000-0000-0000-00000000000d', 'JST',    'JST',    2),
    ('a0000000-0000-0000-0000-00000000000d', 'barrel', 'Barrel', 3),
    ('a0000000-0000-0000-0000-00000000000d', 'USB',    'USB',    4);

-- connector_gender
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-00000000000e', 'male',   'Male',   0),
    ('a0000000-0000-0000-0000-00000000000e', 'female', 'Female', 1);

-- mounting_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-00000000000f', 'through_hole', 'Through-Hole', 0),
    ('a0000000-0000-0000-0000-00000000000f', 'SMD',          'SMD',          1);

-- transistor_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000010', 'NPN',      'NPN',      0),
    ('a0000000-0000-0000-0000-000000000010', 'PNP',      'PNP',      1),
    ('a0000000-0000-0000-0000-000000000010', 'N_MOSFET', 'N-MOSFET', 2),
    ('a0000000-0000-0000-0000-000000000010', 'P_MOSFET', 'P-MOSFET', 3),
    ('a0000000-0000-0000-0000-000000000010', 'JFET',     'JFET',     4);

-- regulator_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000011', 'linear',    'Linear',    0),
    ('a0000000-0000-0000-0000-000000000011', 'switching', 'Switching', 1),
    ('a0000000-0000-0000-0000-000000000011', 'LDO',       'LDO',       2);

-- ============================================================
-- CATEGORIES
-- ============================================================

-- Top-level categories
INSERT INTO categories (id, name, description) VALUES
    ('b0000000-0000-0000-0000-000000000001', 'Resistors',              'Fixed and variable resistors'),
    ('b0000000-0000-0000-0000-000000000002', 'Capacitors',             'All capacitor types'),
    ('b0000000-0000-0000-0000-000000000003', 'Inductors',              'Inductors and chokes'),
    ('b0000000-0000-0000-0000-000000000004', 'Diodes',                 'Rectifiers, Schottky, Zener, LEDs'),
    ('b0000000-0000-0000-0000-000000000010', 'Optoelectronics',        'LEDs, displays, optocouplers'),
    ('b0000000-0000-0000-0000-000000000005', 'Logic ICs',              'Digital logic integrated circuits'),
    ('b0000000-0000-0000-0000-00000000000b', 'Connectors',             'Headers, sockets, and connectors'),
    ('b0000000-0000-0000-0000-00000000000c', 'Transistors',            'BJTs, MOSFETs, JFETs'),
    ('b0000000-0000-0000-0000-00000000000d', 'Voltage Regulators',     'Linear and switching regulators'),
    ('b0000000-0000-0000-0000-00000000000e', 'Crystals & Oscillators', 'Crystals, oscillators, resonators'),
    ('b0000000-0000-0000-0000-00000000000f', 'Misc / Unsorted',        'Uncategorized components');

-- Logic IC child categories
INSERT INTO categories (id, name, parent_id, description) VALUES
    ('b0000000-0000-0000-0000-000000000006', 'Gates',              'b0000000-0000-0000-0000-000000000005', 'AND, OR, NAND, NOR, XOR gates'),
    ('b0000000-0000-0000-0000-000000000007', 'Flip-Flops',         'b0000000-0000-0000-0000-000000000005', 'D, JK, SR, T flip-flops'),
    ('b0000000-0000-0000-0000-000000000008', 'Shift Registers',    'b0000000-0000-0000-0000-000000000005', 'Serial/parallel shift registers'),
    ('b0000000-0000-0000-0000-000000000009', 'Buffers & Drivers',  'b0000000-0000-0000-0000-000000000005', 'Buffer and driver ICs'),
    ('b0000000-0000-0000-0000-00000000000a', 'Counters',           'b0000000-0000-0000-0000-000000000005', 'Binary, decade, up/down counters');

-- Optoelectronic sub categories
INSERT INTO categories (id, name, parent_id, description) VALUES
    ('b0000000-0000-0000-0000-000000000011', 'LEDs',           'b0000000-0000-0000-0000-000000000010', 'Single LEDs'),
    ('b0000000-0000-0000-0000-000000000012', 'Displays',       'b0000000-0000-0000-0000-000000000010', '7-Segment, Bar Graphs'),
    ('b0000000-0000-0000-0000-000000000013', 'Optocouplers',   'b0000000-0000-0000-0000-000000000010', 'a.k.a. Photocouplers, Optoisolators, Optical Isolators');


-- Expanded passive component subcategories
INSERT INTO categories (id, name, parent_id, description) VALUES
    ('b0000000-0000-0000-0000-000000000020', 'Chip Resistors',             'b0000000-0000-0000-0000-000000000001', 'Surface-mount fixed resistors'),
    ('b0000000-0000-0000-0000-000000000021', 'Through-Hole Resistors',     'b0000000-0000-0000-0000-000000000001', 'Axial and radial fixed resistors'),
    ('b0000000-0000-0000-0000-000000000022', 'Variable Resistors',         'b0000000-0000-0000-0000-000000000001', 'Potentiometers, trimmers, rheostats'),
    ('b0000000-0000-0000-0000-000000000023', 'Resistor Networks',          'b0000000-0000-0000-0000-000000000001', 'Array and network resistors'),
    ('b0000000-0000-0000-0000-000000000024', 'Current Sense Resistors',    'b0000000-0000-0000-0000-000000000001', 'Low-ohm shunt resistors'),
    ('b0000000-0000-0000-0000-000000000025', 'Ceramic Capacitors',         'b0000000-0000-0000-0000-000000000002', 'MLCC and ceramic disc capacitors'),
    ('b0000000-0000-0000-0000-000000000026', 'Electrolytic Capacitors',    'b0000000-0000-0000-0000-000000000002', 'Aluminum electrolytic capacitors'),
    ('b0000000-0000-0000-0000-000000000027', 'Tantalum Capacitors',        'b0000000-0000-0000-0000-000000000002', 'Tantalum capacitors'),
    ('b0000000-0000-0000-0000-000000000028', 'Film Capacitors',            'b0000000-0000-0000-0000-000000000002', 'Polyester, polypropylene, and other film capacitors'),
    ('b0000000-0000-0000-0000-000000000029', 'Supercapacitors',            'b0000000-0000-0000-0000-000000000002', 'Electric double-layer capacitors'),
    ('b0000000-0000-0000-0000-00000000002a', 'Power Inductors',            'b0000000-0000-0000-0000-000000000003', 'Shielded and unshielded power inductors'),
    ('b0000000-0000-0000-0000-00000000002b', 'RF Inductors',               'b0000000-0000-0000-0000-000000000003', 'High-frequency inductors'),
    ('b0000000-0000-0000-0000-00000000002c', 'Common Mode Chokes',         'b0000000-0000-0000-0000-000000000003', 'EMI common-mode chokes'),
    ('b0000000-0000-0000-0000-00000000002d', 'Ferrite Beads',              'b0000000-0000-0000-0000-000000000003', 'Ferrite beads and chip EMI filters');

-- Expanded semiconductor categories
INSERT INTO categories (id, name, parent_id, description) VALUES
    ('b0000000-0000-0000-0000-00000000002e', 'Signal Diodes',              'b0000000-0000-0000-0000-000000000004', 'Small-signal switching diodes'),
    ('b0000000-0000-0000-0000-00000000002f', 'Rectifier Diodes',           'b0000000-0000-0000-0000-000000000004', 'Power rectifier diodes and bridges'),
    ('b0000000-0000-0000-0000-000000000030', 'Schottky Diodes',            'b0000000-0000-0000-0000-000000000004', 'Schottky barrier diodes'),
    ('b0000000-0000-0000-0000-000000000031', 'Zener Diodes',               'b0000000-0000-0000-0000-000000000004', 'Voltage reference and clamp diodes'),
    ('b0000000-0000-0000-0000-000000000032', 'TVS Diodes',                 'b0000000-0000-0000-0000-000000000004', 'Transient voltage suppressors'),
    ('b0000000-0000-0000-0000-000000000033', 'Bridge Rectifiers',          'b0000000-0000-0000-0000-00000000002f', 'Four-diode rectifier bridge packages'),
    ('b0000000-0000-0000-0000-000000000034', 'BJTs',                       'b0000000-0000-0000-0000-00000000000c', 'Bipolar junction transistors'),
    ('b0000000-0000-0000-0000-000000000035', 'MOSFETs',                    'b0000000-0000-0000-0000-00000000000c', 'N-channel and P-channel MOSFETs'),
    ('b0000000-0000-0000-0000-000000000036', 'JFETs',                      'b0000000-0000-0000-0000-00000000000c', 'Junction field-effect transistors'),
    ('b0000000-0000-0000-0000-000000000037', 'IGBTs',                      'b0000000-0000-0000-0000-00000000000c', 'Insulated-gate bipolar transistors'),
    ('b0000000-0000-0000-0000-000000000038', 'Darlington Transistors',     'b0000000-0000-0000-0000-00000000000c', 'Darlington transistor pairs and arrays');

-- Integrated circuits
INSERT INTO categories (id, name, parent_id, description) VALUES
    ('b0000000-0000-0000-0000-000000000039', 'Microcontrollers',           NULL, 'MCUs and embedded controllers'),
    ('b0000000-0000-0000-0000-00000000003a', 'Memory ICs',                 NULL, 'RAM, ROM, Flash, EEPROM'),
    ('b0000000-0000-0000-0000-00000000003b', 'Analog ICs',                 NULL, 'Operational amplifiers, comparators, and analog signal ICs'),
    ('b0000000-0000-0000-0000-00000000003c', 'Power Management ICs',       NULL, 'Power conversion and management ICs'),
    ('b0000000-0000-0000-0000-00000000003d', 'Interface ICs',              NULL, 'Bus, line, and protocol interface ICs'),
    ('b0000000-0000-0000-0000-00000000003e', 'Timers & Clocks',            NULL, 'Timers, RTCs, PLLs, and clock generators'),
    ('b0000000-0000-0000-0000-00000000003f', 'Audio ICs',                  NULL, 'Audio amplifiers, codecs, and signal processors'),
    ('b0000000-0000-0000-0000-000000000040', 'RF & Wireless ICs',          NULL, 'RF front ends, radios, and wireless transceivers'),
    ('b0000000-0000-0000-0000-000000000041', '8-bit MCUs',                 'b0000000-0000-0000-0000-000000000039', '8-bit microcontrollers'),
    ('b0000000-0000-0000-0000-000000000042', '32-bit MCUs',                'b0000000-0000-0000-0000-000000000039', 'ARM, RISC-V, and other 32-bit microcontrollers'),
    ('b0000000-0000-0000-0000-000000000043', 'Development Boards',         'b0000000-0000-0000-0000-000000000039', 'MCU development boards and modules'),
    ('b0000000-0000-0000-0000-000000000044', 'SRAM',                       'b0000000-0000-0000-0000-00000000003a', 'Static RAM'),
    ('b0000000-0000-0000-0000-000000000045', 'DRAM',                       'b0000000-0000-0000-0000-00000000003a', 'Dynamic RAM'),
    ('b0000000-0000-0000-0000-000000000046', 'Flash Memory',               'b0000000-0000-0000-0000-00000000003a', 'NOR and NAND flash memory'),
    ('b0000000-0000-0000-0000-000000000047', 'EEPROM',                     'b0000000-0000-0000-0000-00000000003a', 'Electrically erasable programmable memory'),
    ('b0000000-0000-0000-0000-000000000048', 'Op-Amps',                    'b0000000-0000-0000-0000-00000000003b', 'Operational amplifiers'),
    ('b0000000-0000-0000-0000-000000000049', 'Comparators',                'b0000000-0000-0000-0000-00000000003b', 'Analog comparators'),
    ('b0000000-0000-0000-0000-00000000004a', 'ADC & DAC',                  'b0000000-0000-0000-0000-00000000003b', 'Analog-to-digital and digital-to-analog converters'),
    ('b0000000-0000-0000-0000-00000000004b', 'Voltage References',         'b0000000-0000-0000-0000-00000000003b', 'Precision voltage references'),
    ('b0000000-0000-0000-0000-00000000004c', 'DC-DC Converters',           'b0000000-0000-0000-0000-00000000003c', 'Switching converter controller and regulator ICs'),
    ('b0000000-0000-0000-0000-00000000004d', 'Battery Management',         'b0000000-0000-0000-0000-00000000003c', 'Chargers, fuel gauges, and protection ICs'),
    ('b0000000-0000-0000-0000-00000000004e', 'Motor Drivers',              'b0000000-0000-0000-0000-00000000003c', 'Stepper, brushed, and brushless motor drivers'),
    ('b0000000-0000-0000-0000-00000000004f', 'RS-232 / RS-485',            'b0000000-0000-0000-0000-00000000003d', 'Serial line transceivers'),
    ('b0000000-0000-0000-0000-000000000050', 'USB Interface ICs',          'b0000000-0000-0000-0000-00000000003d', 'USB bridges, hubs, and PHYs'),
    ('b0000000-0000-0000-0000-000000000051', 'CAN Interface ICs',          'b0000000-0000-0000-0000-00000000003d', 'CAN controllers and transceivers'),
    ('b0000000-0000-0000-0000-000000000052', 'RTC ICs',                    'b0000000-0000-0000-0000-00000000003e', 'Real-time clocks'),
    ('b0000000-0000-0000-0000-000000000053', 'Clock Generators',           'b0000000-0000-0000-0000-00000000003e', 'Clock generator and PLL ICs');

-- Connectors, electromechanical, sensors, and tooling
INSERT INTO categories (id, name, parent_id, description) VALUES
    ('b0000000-0000-0000-0000-000000000054', 'Headers',                    'b0000000-0000-0000-0000-00000000000b', 'Pin and socket headers'),
    ('b0000000-0000-0000-0000-000000000055', 'Terminal Blocks',            'b0000000-0000-0000-0000-00000000000b', 'Screw and spring terminal blocks'),
    ('b0000000-0000-0000-0000-000000000056', 'Board-to-Board Connectors',  'b0000000-0000-0000-0000-00000000000b', 'Mezzanine and board stacking connectors'),
    ('b0000000-0000-0000-0000-000000000057', 'Wire-to-Board Connectors',   'b0000000-0000-0000-0000-00000000000b', 'JST, Molex, and similar wire connectors'),
    ('b0000000-0000-0000-0000-000000000058', 'USB Connectors',             'b0000000-0000-0000-0000-00000000000b', 'USB receptacles and plugs'),
    ('b0000000-0000-0000-0000-000000000059', 'RF Connectors',              'b0000000-0000-0000-0000-00000000000b', 'SMA, U.FL, BNC, and coax connectors'),
    ('b0000000-0000-0000-0000-00000000005a', 'Switches',                   NULL, 'Tactile, toggle, slide, and rotary switches'),
    ('b0000000-0000-0000-0000-00000000005b', 'Relays',                     NULL, 'Electromechanical and solid-state relays'),
    ('b0000000-0000-0000-0000-00000000005c', 'Sensors',                    NULL, 'Environmental, motion, optical, and position sensors'),
    ('b0000000-0000-0000-0000-00000000005d', 'Modules',                    NULL, 'Assembled modules and breakout boards'),
    ('b0000000-0000-0000-0000-00000000005e', 'Cables & Wire',              NULL, 'Cable assemblies, hookup wire, and ribbon cable'),
    ('b0000000-0000-0000-0000-00000000005f', 'Hardware',                   NULL, 'Standoffs, screws, heatsinks, and mechanical parts'),
    ('b0000000-0000-0000-0000-000000000060', 'Tools & Consumables',        NULL, 'Solder, flux, tools, and workshop supplies'),
    ('b0000000-0000-0000-0000-000000000061', 'Tactile Switches',           'b0000000-0000-0000-0000-00000000005a', 'Momentary tactile switches'),
    ('b0000000-0000-0000-0000-000000000062', 'Slide Switches',             'b0000000-0000-0000-0000-00000000005a', 'Slide switches'),
    ('b0000000-0000-0000-0000-000000000063', 'Toggle Switches',            'b0000000-0000-0000-0000-00000000005a', 'Toggle switches'),
    ('b0000000-0000-0000-0000-000000000064', 'Temperature Sensors',        'b0000000-0000-0000-0000-00000000005c', 'Temperature sensing components'),
    ('b0000000-0000-0000-0000-000000000065', 'Humidity Sensors',           'b0000000-0000-0000-0000-00000000005c', 'Humidity and environmental sensors'),
    ('b0000000-0000-0000-0000-000000000066', 'Motion Sensors',             'b0000000-0000-0000-0000-00000000005c', 'Accelerometers, gyros, and IMUs'),
    ('b0000000-0000-0000-0000-000000000067', 'Pressure Sensors',           'b0000000-0000-0000-0000-00000000005c', 'Pressure and barometric sensors'),
    ('b0000000-0000-0000-0000-000000000068', 'Light Sensors',              'b0000000-0000-0000-0000-00000000005c', 'Photodiodes, phototransistors, and light sensors'),
    ('b0000000-0000-0000-0000-000000000069', 'Distance Sensors',           'b0000000-0000-0000-0000-00000000005c', 'Ultrasonic, ToF, and ranging sensors'),
    ('b0000000-0000-0000-0000-00000000006a', 'Wireless Modules',           'b0000000-0000-0000-0000-00000000005d', 'Wi-Fi, Bluetooth, LoRa, and radio modules'),
    ('b0000000-0000-0000-0000-00000000006b', 'Power Modules',              'b0000000-0000-0000-0000-00000000005d', 'Prebuilt power converter modules'),
    ('b0000000-0000-0000-0000-00000000006c', 'Display Modules',            'b0000000-0000-0000-0000-00000000005d', 'LCD, OLED, and LED display modules'),
    ('b0000000-0000-0000-0000-00000000006d', 'Single Board Computers',     'b0000000-0000-0000-0000-00000000005d', 'Embedded Linux and small computer boards'),
    ('b0000000-0000-0000-0000-00000000006e', 'Hookup Wire',                'b0000000-0000-0000-0000-00000000005e', 'Single-conductor hookup wire'),
    ('b0000000-0000-0000-0000-00000000006f', 'Ribbon Cable',               'b0000000-0000-0000-0000-00000000005e', 'Flat ribbon cable'),
    ('b0000000-0000-0000-0000-000000000070', 'Cable Assemblies',           'b0000000-0000-0000-0000-00000000005e', 'Finished cable assemblies'),
    ('b0000000-0000-0000-0000-000000000071', 'Heatsinks',                  'b0000000-0000-0000-0000-00000000005f', 'Thermal heatsinks'),
    ('b0000000-0000-0000-0000-000000000072', 'Standoffs & Spacers',        'b0000000-0000-0000-0000-00000000005f', 'PCB standoffs and spacers'),
    ('b0000000-0000-0000-0000-000000000073', 'Enclosures',                 'b0000000-0000-0000-0000-00000000005f', 'Project boxes and enclosures'),
    ('b0000000-0000-0000-0000-000000000074', 'Solder & Flux',              'b0000000-0000-0000-0000-000000000060', 'Solder wire, paste, and flux'),
    ('b0000000-0000-0000-0000-000000000075', 'Prototyping Boards',         'b0000000-0000-0000-0000-000000000060', 'Breadboards, perfboard, and protoboard'),
    ('b0000000-0000-0000-0000-000000000076', 'Test Leads & Probes',        'b0000000-0000-0000-0000-000000000060', 'Meter leads, clips, and scope probes');

-- ============================================================
-- ATTRIBUTE DEFINITIONS
-- ============================================================

-- Resistors: resistance, tolerance, power_rating, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000001', 'resistance',   'Resistance',   'numeric', 'Ω', NULL, true,  0),
    ('b0000000-0000-0000-0000-000000000001', 'tolerance',    'Tolerance',    'enum',    NULL, 'a0000000-0000-0000-0000-000000000003', false, 10),
    ('b0000000-0000-0000-0000-000000000001', 'power_rating', 'Power Rating', 'numeric', 'W',  NULL, false, 20),
    ('b0000000-0000-0000-0000-000000000001', 'package',      'Package',      'enum',    NULL, 'a0000000-0000-0000-0000-000000000001', false, 30);

-- Capacitors: capacitance, voltage_rating, capacitor_type, dielectric, tolerance, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000002', 'capacitance',     'Capacitance',     'numeric', 'F',  NULL, true,  0),
    ('b0000000-0000-0000-0000-000000000002', 'voltage_rating',  'Voltage Rating',  'numeric', 'V',  NULL, false, 10),
    ('b0000000-0000-0000-0000-000000000002', 'capacitor_type',  'Capacitor Type',  'enum',    NULL, 'a0000000-0000-0000-0000-000000000004', false, 20),
    ('b0000000-0000-0000-0000-000000000002', 'dielectric',      'Dielectric',      'enum',    NULL, 'a0000000-0000-0000-0000-000000000005', false, 30),
    ('b0000000-0000-0000-0000-000000000002', 'tolerance',       'Tolerance',       'enum',    NULL, 'a0000000-0000-0000-0000-000000000003', false, 40),
    ('b0000000-0000-0000-0000-000000000002', 'package',         'Package',         'enum',    NULL, 'a0000000-0000-0000-0000-000000000001', false, 50);

-- Inductors: inductance, current_rating, dcr, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000003', 'inductance',     'Inductance',     'numeric', 'H',  NULL, true,  0),
    ('b0000000-0000-0000-0000-000000000003', 'current_rating', 'Current Rating', 'numeric', 'A',  NULL, false, 10),
    ('b0000000-0000-0000-0000-000000000003', 'dcr',            'DCR',            'numeric', 'Ω', NULL, false, 20),
    ('b0000000-0000-0000-0000-000000000003', 'package',        'Package',        'enum',    NULL, 'a0000000-0000-0000-0000-000000000001', false, 30);

-- Diodes: forward_voltage, max_current, reverse_voltage, type, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000004', 'forward_voltage',  'Forward Voltage',  'numeric', 'V',  NULL, false, 0),
    ('b0000000-0000-0000-0000-000000000004', 'max_current',      'Max Current',      'numeric', 'A',  NULL, false, 10),
    ('b0000000-0000-0000-0000-000000000004', 'reverse_voltage',  'Reverse Voltage',  'numeric', 'V',  NULL, false, 20),
    ('b0000000-0000-0000-0000-000000000004', 'diode_type',       'Type',             'enum',    NULL, 'a0000000-0000-0000-0000-000000000007', false, 30),
    ('b0000000-0000-0000-0000-000000000004', 'package',          'Package',          'enum',    NULL, 'a0000000-0000-0000-0000-000000000001', false, 40);

-- Logic ICs (parent): logic_family, supply_voltage_min, supply_voltage_max, propagation_delay, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000005', 'logic_family',       'Logic Family',       'enum',    NULL,  'a0000000-0000-0000-0000-000000000002', false, 0),
    ('b0000000-0000-0000-0000-000000000005', 'supply_voltage_min', 'Supply Voltage Min', 'numeric', 'V',   NULL, false, 10),
    ('b0000000-0000-0000-0000-000000000005', 'supply_voltage_max', 'Supply Voltage Max', 'numeric', 'V',   NULL, false, 20),
    ('b0000000-0000-0000-0000-000000000005', 'propagation_delay',  'Propagation Delay',  'numeric', 'ns',  NULL, false, 30),
    ('b0000000-0000-0000-0000-000000000005', 'package',            'Package',            'enum',    NULL,  'a0000000-0000-0000-0000-000000000001', false, 40);

-- Gates: gate_function, num_gates
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000006', 'gate_function', 'Gate Function', 'enum',    NULL, 'a0000000-0000-0000-0000-000000000006', false, 0),
    ('b0000000-0000-0000-0000-000000000006', 'num_gates',     'Number of Gates', 'numeric', NULL, NULL, false, 10);

-- Flip-Flops: type, num_elements, edge
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000007', 'flipflop_type', 'Type',              'enum',    NULL, 'a0000000-0000-0000-0000-000000000008', false, 0),
    ('b0000000-0000-0000-0000-000000000007', 'num_elements',  'Number of Elements','numeric', NULL, NULL, false, 10),
    ('b0000000-0000-0000-0000-000000000007', 'edge',          'Edge',              'enum',    NULL, 'a0000000-0000-0000-0000-000000000009', false, 20);

-- Shift Registers: num_bits, direction
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000008', 'num_bits',  'Number of Bits', 'numeric', NULL, NULL, false, 0),
    ('b0000000-0000-0000-0000-000000000008', 'direction', 'Direction',      'enum',    NULL, 'a0000000-0000-0000-0000-00000000000a', false, 10);

-- Buffers & Drivers: num_channels, tri_state, direction
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000009', 'num_channels', 'Number of Channels', 'numeric', NULL, NULL, false, 0),
    ('b0000000-0000-0000-0000-000000000009', 'tri_state',    'Tri-State',          'boolean', NULL, NULL, false, 10),
    ('b0000000-0000-0000-0000-000000000009', 'direction',    'Direction',          'enum',    NULL, 'a0000000-0000-0000-0000-00000000000b', false, 20);

-- Counters: num_bits, type
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-00000000000a', 'num_bits',     'Number of Bits', 'numeric', NULL, NULL, false, 0),
    ('b0000000-0000-0000-0000-00000000000a', 'counter_type', 'Type',           'enum',    NULL, 'a0000000-0000-0000-0000-00000000000c', false, 10);

-- Connectors: num_pins, pitch, type, gender, mounting
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-00000000000b', 'num_pins',       'Number of Pins', 'numeric', NULL,  NULL, false, 0),
    ('b0000000-0000-0000-0000-00000000000b', 'pitch',          'Pitch',          'numeric', 'mm',  NULL, false, 10),
    ('b0000000-0000-0000-0000-00000000000b', 'connector_type', 'Type',           'enum',    NULL,  'a0000000-0000-0000-0000-00000000000d', false, 20),
    ('b0000000-0000-0000-0000-00000000000b', 'gender',         'Gender',         'enum',    NULL,  'a0000000-0000-0000-0000-00000000000e', false, 30),
    ('b0000000-0000-0000-0000-00000000000b', 'mounting',       'Mounting',       'enum',    NULL,  'a0000000-0000-0000-0000-00000000000f', false, 40);

-- Transistors: type, max_current, max_voltage, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-00000000000c', 'transistor_type', 'Type',        'enum',    NULL, 'a0000000-0000-0000-0000-000000000010', false, 0),
    ('b0000000-0000-0000-0000-00000000000c', 'max_current',     'Max Current', 'numeric', 'A',  NULL, false, 10),
    ('b0000000-0000-0000-0000-00000000000c', 'max_voltage',     'Max Voltage', 'numeric', 'V',  NULL, false, 20),
    ('b0000000-0000-0000-0000-00000000000c', 'package',         'Package',     'enum',    NULL, 'a0000000-0000-0000-0000-000000000001', false, 30);

-- Voltage Regulators: output_voltage, output_current, type, dropout_voltage, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-00000000000d', 'output_voltage',  'Output Voltage',  'numeric', 'V',  NULL, false, 0),
    ('b0000000-0000-0000-0000-00000000000d', 'output_current',  'Output Current',  'numeric', 'A',  NULL, false, 10),
    ('b0000000-0000-0000-0000-00000000000d', 'regulator_type',  'Type',            'enum',    NULL, 'a0000000-0000-0000-0000-000000000011', false, 20),
    ('b0000000-0000-0000-0000-00000000000d', 'dropout_voltage', 'Dropout Voltage', 'numeric', 'V',  NULL, false, 30),
    ('b0000000-0000-0000-0000-00000000000d', 'package',         'Package',         'enum',    NULL, 'a0000000-0000-0000-0000-000000000001', false, 40);

-- Crystals & Oscillators: frequency, load_capacitance, tolerance, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-00000000000e', 'frequency',        'Frequency',        'numeric', 'Hz',  NULL, true,  0),
    ('b0000000-0000-0000-0000-00000000000e', 'load_capacitance', 'Load Capacitance', 'numeric', 'F',   NULL, false, 10),
    ('b0000000-0000-0000-0000-00000000000e', 'tolerance',        'Tolerance',        'numeric', 'ppm', NULL, false, 20),
    ('b0000000-0000-0000-0000-00000000000e', 'package',          'Package',          'enum',    NULL,  'a0000000-0000-0000-0000-000000000001', false, 30);
