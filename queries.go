package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

var searchNormalizer = strings.NewReplacer("-", " ", "_", " ")

// normalizeQuery replaces punctuation that users and datasheets treat
// interchangeably (hyphens, underscores) with spaces, then collapses
// any resulting runs of spaces so the ILIKE pattern stays clean.
func normalizeQuery(q string) string {
	q = searchNormalizer.Replace(q)
	// Collapse consecutive spaces produced by the replacements above
	for strings.Contains(q, "  ") {
		q = strings.ReplaceAll(q, "  ", " ")
	}
	return strings.TrimSpace(q)
}

// --- Categories ---

func listCategories(db *sql.DB) ([]CategoryListItem, error) {
	rows, err := db.Query(`
		SELECT c.id, c.name, c.parent_id, p.name, c.description,
		       (SELECT COUNT(*) FROM attribute_definitions WHERE category_id = c.id)
		FROM categories c
		LEFT JOIN categories p ON c.parent_id = p.id
		ORDER BY COALESCE(c.parent_id, c.id), c.parent_id NULLS FIRST, c.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CategoryListItem
	for rows.Next() {
		var item CategoryListItem
		if err := rows.Scan(&item.ID, &item.Name, &item.ParentID, &item.ParentName, &item.Description, &item.AttrCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func getCategory(db *sql.DB, id string) (*Category, error) {
	var c Category
	err := db.QueryRow(`
		SELECT id, name, parent_id, description, created_at, updated_at
		FROM categories WHERE id = $1
	`, id).Scan(&c.ID, &c.Name, &c.ParentID, &c.Description, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func createCategory(db *sql.DB, name string, parentID *string, description *string) (string, error) {
	var id string
	err := db.QueryRow(`
		INSERT INTO categories (name, parent_id, description)
		VALUES ($1, $2, $3) RETURNING id
	`, name, parentID, description).Scan(&id)
	return id, err
}

func updateCategory(db *sql.DB, id, name string, parentID *string, description *string) error {
	_, err := db.Exec(`
		UPDATE categories SET name = $2, parent_id = $3, description = $4, updated_at = now()
		WHERE id = $1
	`, id, name, parentID, description)
	return err
}

func deleteCategory(db *sql.DB, id string) error {
	// Check for components first
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM components WHERE category_id = $1`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("cannot delete category: %d components still reference it", count)
	}

	// Check for child categories
	if err := db.QueryRow(`SELECT COUNT(*) FROM categories WHERE parent_id = $1`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("cannot delete category: %d child categories still reference it", count)
	}

	_, err := db.Exec(`DELETE FROM categories WHERE id = $1`, id)
	return err
}

func listParentCandidates(db *sql.DB, excludeID string) ([]Category, error) {
	rows, err := db.Query(`
		SELECT id, name, parent_id, description, created_at, updated_at
		FROM categories
		WHERE parent_id IS NULL AND id != $1
		ORDER BY name
	`, excludeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.ParentID, &c.Description, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

// --- Attribute Definitions ---

func listAttributesByCategory(db *sql.DB, categoryID string) ([]AttributeDefinition, error) {
	rows, err := db.Query(`
		SELECT ad.id, ad.category_id, ad.name, ad.display_name, ad.data_type,
		       ad.unit, ad.enum_group_id, eg.name, ad.is_required, ad.sort_order,
		       ad.created_at, ad.updated_at
		FROM attribute_definitions ad
		LEFT JOIN enum_groups eg ON ad.enum_group_id = eg.id
		WHERE ad.category_id = $1
		ORDER BY ad.sort_order, ad.name
	`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attrs []AttributeDefinition
	for rows.Next() {
		var a AttributeDefinition
		if err := rows.Scan(&a.ID, &a.CategoryID, &a.Name, &a.DisplayName, &a.DataType,
			&a.Unit, &a.EnumGroupID, &a.EnumGroupName, &a.IsRequired, &a.SortOrder,
			&a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		attrs = append(attrs, a)
	}
	return attrs, rows.Err()
}

func createAttribute(db *sql.DB, a AttributeDefinition) (string, error) {
	var id string
	err := db.QueryRow(`
		INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id
	`, a.CategoryID, a.Name, a.DisplayName, a.DataType, a.Unit, a.EnumGroupID, a.IsRequired, a.SortOrder).Scan(&id)
	return id, err
}

func deleteAttribute(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM attribute_definitions WHERE id = $1`, id)
	return err
}

// --- Enum Groups ---

func listEnumGroupsWithValues(db *sql.DB) ([]EnumGroupWithValues, error) {
	groups := make([]EnumGroupWithValues, 0)
	groupRows, err := db.Query(`SELECT id, name, created_at FROM enum_groups ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer groupRows.Close()

	for groupRows.Next() {
		var g EnumGroupWithValues
		if err := groupRows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	if err := groupRows.Err(); err != nil {
		return nil, err
	}

	// Fetch all values in one query
	valueRows, err := db.Query(`
		SELECT id, enum_group_id, value, display_name, sort_order, created_at
		FROM enum_values ORDER BY sort_order, value
	`)
	if err != nil {
		return nil, err
	}
	defer valueRows.Close()

	valuesByGroup := make(map[string][]EnumValue)
	for valueRows.Next() {
		var v EnumValue
		if err := valueRows.Scan(&v.ID, &v.EnumGroupID, &v.Value, &v.DisplayName, &v.SortOrder, &v.CreatedAt); err != nil {
			return nil, err
		}
		valuesByGroup[v.EnumGroupID] = append(valuesByGroup[v.EnumGroupID], v)
	}
	if err := valueRows.Err(); err != nil {
		return nil, err
	}

	for i := range groups {
		groups[i].Values = valuesByGroup[groups[i].ID]
	}

	return groups, nil
}

func getEnumGroup(db *sql.DB, id string) (*EnumGroupWithValues, error) {
	var g EnumGroupWithValues
	err := db.QueryRow(`SELECT id, name, created_at FROM enum_groups WHERE id = $1`, id).Scan(&g.ID, &g.Name, &g.CreatedAt)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT id, enum_group_id, value, display_name, sort_order, created_at
		FROM enum_values WHERE enum_group_id = $1 ORDER BY sort_order, value
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var v EnumValue
		if err := rows.Scan(&v.ID, &v.EnumGroupID, &v.Value, &v.DisplayName, &v.SortOrder, &v.CreatedAt); err != nil {
			return nil, err
		}
		g.Values = append(g.Values, v)
	}
	return &g, rows.Err()
}

func listEnumGroupsSimple(db *sql.DB) ([]EnumGroup, error) {
	rows, err := db.Query(`SELECT id, name, created_at FROM enum_groups ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []EnumGroup
	for rows.Next() {
		var g EnumGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func createEnumGroup(db *sql.DB, name string) (string, error) {
	var id string
	err := db.QueryRow(`INSERT INTO enum_groups (name) VALUES ($1) RETURNING id`, name).Scan(&id)
	return id, err
}

func deleteEnumGroup(db *sql.DB, id string) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM attribute_definitions WHERE enum_group_id = $1`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("cannot delete enum group: %d attribute definitions reference it", count)
	}
	_, err := db.Exec(`DELETE FROM enum_groups WHERE id = $1`, id)
	return err
}

func createEnumValue(db *sql.DB, groupID, value, displayName string) (string, error) {
	var id string
	err := db.QueryRow(`
		INSERT INTO enum_values (enum_group_id, value, display_name)
		VALUES ($1, $2, $3) RETURNING id
	`, groupID, value, displayName).Scan(&id)
	return id, err
}

func deleteEnumValue(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM enum_values WHERE id = $1`, id)
	return err
}

// --- Components ---

func listComponents(db *sql.DB, categoryID string, q string, filters []AttrFilter) ([]ComponentListItem, error) {
	query := `
		SELECT c.id, c.category_id, cat.name, c.mpn, c.manufacturer, c.description,
		       c.quantity, c.min_quantity, c.location_id, sl.name, c.updated_at
		FROM components c
		JOIN categories cat ON c.category_id = cat.id
		LEFT JOIN storage_locations sl ON c.location_id = sl.id
	`
	var args []any
	argN := 1
	var conditions []string

	if categoryID != "" {
		conditions = append(conditions, fmt.Sprintf("(c.category_id = $%d OR c.category_id IN (SELECT id FROM categories WHERE parent_id = $%d))", argN, argN))
		args = append(args, categoryID)
		argN++
	}

	if q != "" {
		pattern := "%" + normalizeQuery(q) + "%"
		// Normalize column values the same way so "flip-flop" matches "flip flop" and vice versa
		conditions = append(conditions, fmt.Sprintf(
			`(REPLACE(REPLACE(c.mpn,        '-', ' '), '_', ' ') ILIKE $%d OR
			  REPLACE(REPLACE(c.manufacturer,'-', ' '), '_', ' ') ILIKE $%d OR
			  REPLACE(REPLACE(c.description, '-', ' '), '_', ' ') ILIKE $%d OR
			  REPLACE(REPLACE(c.notes,       '-', ' '), '_', ' ') ILIKE $%d)`,
			argN, argN, argN, argN))
		args = append(args, pattern)
		argN++
	}

	for _, f := range filters {
		switch f.DataType {
		case "numeric":
			if f.MinValue != nil {
				conditions = append(conditions, fmt.Sprintf(
					"EXISTS (SELECT 1 FROM component_attributes ca WHERE ca.component_id = c.id AND ca.attribute_definition_id = $%d AND ca.value_numeric >= $%d)",
					argN, argN+1))
				args = append(args, f.AttrDefID, *f.MinValue)
				argN += 2
			}
			if f.MaxValue != nil {
				conditions = append(conditions, fmt.Sprintf(
					"EXISTS (SELECT 1 FROM component_attributes ca WHERE ca.component_id = c.id AND ca.attribute_definition_id = $%d AND ca.value_numeric <= $%d)",
					argN, argN+1))
				args = append(args, f.AttrDefID, *f.MaxValue)
				argN += 2
			}
		case "enum":
			if len(f.EnumValues) > 0 {
				placeholders := ""
				for i, ev := range f.EnumValues {
					if i > 0 {
						placeholders += ", "
					}
					placeholders += fmt.Sprintf("$%d", argN)
					args = append(args, ev)
					argN++
				}
				conditions = append(conditions, fmt.Sprintf(
					"EXISTS (SELECT 1 FROM component_attributes ca WHERE ca.component_id = c.id AND ca.attribute_definition_id = $%d AND ca.value_enum IN (%s))",
					argN, placeholders))
				args = append(args, f.AttrDefID)
				argN++
			}
		case "text":
			if f.TextSearch != "" {
				conditions = append(conditions, fmt.Sprintf(
					"EXISTS (SELECT 1 FROM component_attributes ca WHERE ca.component_id = c.id AND ca.attribute_definition_id = $%d AND ca.value_text ILIKE $%d)",
					argN, argN+1))
				args = append(args, f.AttrDefID, "%"+f.TextSearch+"%")
				argN += 2
			}
		case "boolean":
			if f.BoolValue != nil {
				conditions = append(conditions, fmt.Sprintf(
					"EXISTS (SELECT 1 FROM component_attributes ca WHERE ca.component_id = c.id AND ca.attribute_definition_id = $%d AND ca.value_boolean = $%d)",
					argN, argN+1))
				args = append(args, f.AttrDefID, *f.BoolValue)
				argN += 2
			}
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + conditions[0]
		for _, c := range conditions[1:] {
			query += " AND " + c
		}
	}

	query += ` ORDER BY c.updated_at DESC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ComponentListItem
	for rows.Next() {
		var item ComponentListItem
		if err := rows.Scan(&item.ID, &item.CategoryID, &item.CategoryName,
			&item.MPN, &item.Manufacturer, &item.Description,
			&item.Quantity, &item.MinQuantity, &item.LocationID, &item.LocationName, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func getComponent(db *sql.DB, id string) (*Component, error) {
	var c Component
	err := db.QueryRow(`
		SELECT c.id, c.category_id, cat.name, c.mpn, c.manufacturer, c.description,
		       c.quantity, c.min_quantity, c.location_id, sl.name,
		       c.datasheet_url, c.notes, c.created_at, c.updated_at
		FROM components c
		JOIN categories cat ON c.category_id = cat.id
		LEFT JOIN storage_locations sl ON c.location_id = sl.id
		WHERE c.id = $1
	`, id).Scan(&c.ID, &c.CategoryID, &c.CategoryName, &c.MPN, &c.Manufacturer,
		&c.Description, &c.Quantity, &c.MinQuantity, &c.LocationID, &c.LocationName,
		&c.DatasheetURL, &c.Notes, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func createComponent(db *sql.DB, c *Component) (string, error) {
	var id string
	err := db.QueryRow(`
		INSERT INTO components (category_id, mpn, manufacturer, description, quantity, min_quantity, location_id, datasheet_url, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id
	`, c.CategoryID, c.MPN, c.Manufacturer, c.Description,
		c.Quantity, c.MinQuantity, c.LocationID, c.DatasheetURL, c.Notes).Scan(&id)
	return id, err
}

func updateComponent(db *sql.DB, c *Component) error {
	_, err := db.Exec(`
		UPDATE components SET mpn = $2, manufacturer = $3, description = $4,
		       quantity = $5, min_quantity = $6, location_id = $7, datasheet_url = $8,
		       notes = $9, updated_at = now()
		WHERE id = $1
	`, c.ID, c.MPN, c.Manufacturer, c.Description,
		c.Quantity, c.MinQuantity, c.LocationID, c.DatasheetURL, c.Notes)
	return err
}

func deleteComponent(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM components WHERE id = $1`, id)
	return err
}

func adjustComponentQuantity(db *sql.DB, id string, delta int) (int, error) {
	var newQty int
	err := db.QueryRow(`
		UPDATE components SET quantity = GREATEST(quantity + $2, 0), updated_at = now()
		WHERE id = $1 RETURNING quantity
	`, id, delta).Scan(&newQty)
	return newQty, err
}

// --- Component Attributes ---

func getComponentAttributes(db *sql.DB, componentID string) ([]ComponentAttribute, error) {
	rows, err := db.Query(`
		SELECT ca.id, ca.component_id, ca.attribute_definition_id,
		       ca.value_numeric, ca.value_text, ca.value_enum, ca.value_boolean,
		       ad.name, ad.display_name, ad.data_type, ad.unit,
		       ev.display_name
		FROM component_attributes ca
		JOIN attribute_definitions ad ON ca.attribute_definition_id = ad.id
		LEFT JOIN enum_values ev ON ca.value_enum = ev.id
		WHERE ca.component_id = $1
		ORDER BY ad.sort_order, ad.name
	`, componentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attrs []ComponentAttribute
	for rows.Next() {
		var a ComponentAttribute
		if err := rows.Scan(&a.ID, &a.ComponentID, &a.AttributeDefinitionID,
			&a.ValueNumeric, &a.ValueText, &a.ValueEnum, &a.ValueBoolean,
			&a.AttrName, &a.AttrDisplayName, &a.AttrDataType, &a.AttrUnit,
			&a.EnumDisplayName); err != nil {
			return nil, err
		}
		attrs = append(attrs, a)
	}
	return attrs, rows.Err()
}

func saveComponentAttributes(db *sql.DB, componentID string, attrs []ComponentAttribute) error {
	// Delete existing attributes and re-insert
	if _, err := db.Exec(`DELETE FROM component_attributes WHERE component_id = $1`, componentID); err != nil {
		return err
	}

	for _, a := range attrs {
		_, err := db.Exec(`
			INSERT INTO component_attributes (component_id, attribute_definition_id, value_numeric, value_text, value_enum, value_boolean)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, componentID, a.AttributeDefinitionID, a.ValueNumeric, a.ValueText, a.ValueEnum, a.ValueBoolean)
		if err != nil {
			return err
		}
	}
	return nil
}

// getMergedAttributes returns attribute definitions for a category, including inherited parent attrs.
func getMergedAttributes(db *sql.DB, categoryID string) ([]AttributeDefinition, error) {
	cat, err := getCategory(db, categoryID)
	if err != nil {
		return nil, err
	}

	var allAttrs []AttributeDefinition

	// If this category has a parent, get parent attrs first
	if cat.ParentID != nil {
		parentAttrs, err := listAttributesByCategory(db, *cat.ParentID)
		if err != nil {
			return nil, err
		}
		allAttrs = append(allAttrs, parentAttrs...)
	}

	// Then get this category's own attrs
	ownAttrs, err := listAttributesByCategory(db, categoryID)
	if err != nil {
		return nil, err
	}
	allAttrs = append(allAttrs, ownAttrs...)

	return allAttrs, nil
}

// listAllCategories returns all categories for the sidebar, including parents and leaves.
func listAllCategories(db *sql.DB) ([]CategoryListItem, error) {
	rows, err := db.Query(`
		SELECT c.id, c.name, c.parent_id, p.name, c.description,
		       (SELECT COUNT(*) FROM attribute_definitions WHERE category_id = c.id),
		       EXISTS (SELECT 1 FROM categories child WHERE child.parent_id = c.id)
		FROM categories c
		LEFT JOIN categories p ON c.parent_id = p.id
		ORDER BY COALESCE(p.name, c.name), c.parent_id IS NOT NULL, c.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CategoryListItem
	for rows.Next() {
		var item CategoryListItem
		if err := rows.Scan(&item.ID, &item.Name, &item.ParentID, &item.ParentName, &item.Description, &item.AttrCount, &item.HasChildren); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// getEnumValuesForGroup returns all values for a given enum group.
func getEnumValuesForGroup(db *sql.DB, groupID string) ([]EnumValue, error) {
	rows, err := db.Query(`
		SELECT id, enum_group_id, value, display_name, sort_order, created_at
		FROM enum_values WHERE enum_group_id = $1 ORDER BY sort_order, value
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vals []EnumValue
	for rows.Next() {
		var v EnumValue
		if err := rows.Scan(&v.ID, &v.EnumGroupID, &v.Value, &v.DisplayName, &v.SortOrder, &v.CreatedAt); err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	return vals, rows.Err()
}

// --- Dashboard ---

func getDashboardStats(db *sql.DB) (*DashboardStats, error) {
	var s DashboardStats
	err := db.QueryRow(`
		SELECT
			COUNT(*),
			COUNT(DISTINCT category_id),
			COALESCE(SUM(quantity), 0),
			COUNT(*) FILTER (WHERE min_quantity > 0 AND quantity <= min_quantity)
		FROM components
	`).Scan(&s.TotalComponents, &s.UniqueCategories, &s.TotalQuantity, &s.LowStockCount)
	return &s, err
}

func getLowStockComponents(db *sql.DB) ([]LowStockItem, error) {
	rows, err := db.Query(`
		SELECT c.id, c.mpn, c.manufacturer, cat.name, c.quantity, c.min_quantity, sl.name
		FROM components c
		JOIN categories cat ON c.category_id = cat.id
		LEFT JOIN storage_locations sl ON c.location_id = sl.id
		WHERE c.min_quantity > 0 AND c.quantity <= c.min_quantity
		ORDER BY c.quantity - c.min_quantity, c.quantity
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []LowStockItem
	for rows.Next() {
		var item LowStockItem
		if err := rows.Scan(&item.ID, &item.MPN, &item.Manufacturer, &item.CategoryName,
			&item.Quantity, &item.MinQuantity, &item.LocationName); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func getRecentComponents(db *sql.DB, limit int) ([]ComponentListItem, error) {
	rows, err := db.Query(`
		SELECT c.id, c.category_id, cat.name, c.mpn, c.manufacturer, c.description,
		       c.quantity, c.min_quantity, c.location_id, sl.name, c.updated_at
		FROM components c
		JOIN categories cat ON c.category_id = cat.id
		LEFT JOIN storage_locations sl ON c.location_id = sl.id
		ORDER BY c.updated_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ComponentListItem
	for rows.Next() {
		var item ComponentListItem
		if err := rows.Scan(&item.ID, &item.CategoryID, &item.CategoryName,
			&item.MPN, &item.Manufacturer, &item.Description,
			&item.Quantity, &item.MinQuantity, &item.LocationID, &item.LocationName, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// --- Storage Locations ---

func listStorageLocations(db *sql.DB) ([]StorageLocation, error) {
	rows, err := db.Query(`
		SELECT sl.id, sl.name, sl.parent_id, p.name, sl.description, sl.barcode,
		       COUNT(c.id), sl.created_at
		FROM storage_locations sl
		LEFT JOIN storage_locations p ON sl.parent_id = p.id
		LEFT JOIN components c ON c.location_id = sl.id
		GROUP BY sl.id, sl.name, sl.parent_id, p.name, sl.description, sl.barcode, sl.created_at
		ORDER BY COALESCE(p.name, sl.name), sl.parent_id IS NOT NULL, sl.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []StorageLocation
	for rows.Next() {
		var loc StorageLocation
		if err := rows.Scan(&loc.ID, &loc.Name, &loc.ParentID, &loc.ParentName,
			&loc.Description, &loc.Barcode, &loc.ComponentCount, &loc.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, loc)
	}
	return items, rows.Err()
}

func getStorageLocation(db *sql.DB, id string) (*StorageLocation, error) {
	var loc StorageLocation
	err := db.QueryRow(`
		SELECT sl.id, sl.name, sl.parent_id, p.name, sl.description, sl.barcode, 0, sl.created_at
		FROM storage_locations sl
		LEFT JOIN storage_locations p ON sl.parent_id = p.id
		WHERE sl.id = $1
	`, id).Scan(&loc.ID, &loc.Name, &loc.ParentID, &loc.ParentName,
		&loc.Description, &loc.Barcode, &loc.ComponentCount, &loc.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &loc, nil
}

func createStorageLocation(db *sql.DB, name string, parentID *string, description *string) (*StorageLocation, error) {
	var loc StorageLocation
	err := db.QueryRow(`
		INSERT INTO storage_locations (name, parent_id, description, barcode)
		VALUES ($1, $2, $3, upper(substring(gen_random_uuid()::text, 1, 8)))
		RETURNING id, name, parent_id, NULL, description, barcode, created_at
	`, name, parentID, description).Scan(
		&loc.ID, &loc.Name, &loc.ParentID, &loc.ParentName,
		&loc.Description, &loc.Barcode, &loc.CreatedAt)
	return &loc, err
}

func updateStorageLocation(db *sql.DB, id string, name string, parentID *string, description *string) error {
	_, err := db.Exec(`
		UPDATE storage_locations SET name = $2, parent_id = $3, description = $4
		WHERE id = $1
	`, id, name, parentID, description)
	return err
}

func deleteStorageLocation(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM storage_locations WHERE id = $1`, id)
	return err
}

// --- Audit Log ---

func insertAuditLog(db *sql.DB, tableName, recordID, action string, oldValues, newValues any) {
	var oldJSON, newJSON *string
	if oldValues != nil {
		b, err := json.Marshal(oldValues)
		if err == nil {
			s := string(b)
			oldJSON = &s
		}
	}
	if newValues != nil {
		b, err := json.Marshal(newValues)
		if err == nil {
			s := string(b)
			newJSON = &s
		}
	}

	_, err := db.Exec(`
		INSERT INTO audit_log (table_name, record_id, action, old_values, new_values)
		VALUES ($1, $2, $3, $4, $5)
	`, tableName, recordID, action, oldJSON, newJSON)
	if err != nil {
		log.Printf("error inserting audit log: %v", err)
	}
}

func listAuditLog(db *sql.DB, limit, offset int) ([]AuditLogEntry, int, error) {
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`
		SELECT id, table_name, record_id, action, old_values, new_values, created_at
		FROM audit_log
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []AuditLogEntry
	for rows.Next() {
		var e AuditLogEntry
		if err := rows.Scan(&e.ID, &e.TableName, &e.RecordID, &e.Action,
			&e.OldValues, &e.NewValues, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}
