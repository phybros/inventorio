ALTER TABLE components ADD COLUMN location_id UUID REFERENCES storage_locations(id) ON DELETE SET NULL;
