CREATE INDEX IF NOT EXISTS idx_components_manufacturer
    ON components USING gin (manufacturer gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_components_description
    ON components USING gin (description gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_components_notes
    ON components USING gin (notes gin_trgm_ops);
