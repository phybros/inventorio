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
