package main

import "time"

type Category struct {
	ID          string
	Name        string
	ParentID    *string
	Path        string
	Depth       int
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CategoryListItem struct {
	ID          string
	Name        string
	ParentID    *string
	ParentName  *string
	Path        string
	Depth       int
	Description *string
	AttrCount   int
	HasChildren bool
}

type AttributeDefinition struct {
	ID            string
	CategoryID    string
	Name          string
	DisplayName   string
	DataType      string // "numeric", "text", "enum", "boolean"
	Unit          *string
	EnumGroupID   *string
	EnumGroupName *string // populated by joins
	IsRequired    bool
	SortOrder     int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type EnumGroup struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type EnumGroupWithValues struct {
	ID        string
	Name      string
	CreatedAt time.Time
	Values    []EnumValue
}

type EnumValue struct {
	ID          string
	EnumGroupID string
	Value       string
	DisplayName string
	SortOrder   int
	CreatedAt   time.Time
}

type StorageLocation struct {
	ID             string
	Name           string
	ParentID       *string
	ParentName     *string
	Description    *string
	Barcode        string
	ComponentCount int
	CreatedAt      time.Time
}

type Component struct {
	ID           string
	CategoryID   string
	CategoryName string
	MPN          *string
	Manufacturer *string
	Description  *string
	Quantity     int
	MinQuantity  int
	LocationID   *string
	LocationName *string
	DatasheetURL *string
	Notes        *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ComponentAttribute struct {
	ID                    string
	ComponentID           string
	AttributeDefinitionID string
	ValueNumeric          *float64
	ValueText             *string
	ValueEnum             *string // enum_value ID
	ValueBoolean          *bool
	// Joined fields for display
	AttrName        string
	AttrDisplayName string
	AttrDataType    string
	AttrUnit        *string
	EnumDisplayName *string // resolved enum value display name
}

type ComponentListItem struct {
	ID           string
	CategoryID   string
	CategoryName string
	MPN          *string
	Manufacturer *string
	Description  *string
	Quantity     int
	MinQuantity  int
	LocationID   *string
	LocationName *string
	UpdatedAt    time.Time
}

type DashboardStats struct {
	TotalComponents  int
	UniqueCategories int
	TotalQuantity    int
	LowStockCount    int
}

type LowStockItem struct {
	ID           string
	MPN          *string
	Manufacturer *string
	CategoryName string
	Quantity     int
	MinQuantity  int
	LocationName *string
}

type AttrFilter struct {
	AttrDefID  string
	DataType   string
	MinValue   *float64
	MaxValue   *float64
	EnumValues []string // selected enum value IDs
	TextSearch string
	BoolValue  *bool
}

type Project struct {
	ID          string
	Name        string
	Status      string // "draft", "active", "archived"
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProjectListItem struct {
	ID            string
	Name          string
	Status        string
	Description   *string
	BOMItemCount  int
	Buildable     bool
	ShortageCount int
	BuildCount    int
	UpdatedAt     time.Time
}

type BOMItem struct {
	ID          string
	ProjectID   string
	ComponentID string
	Quantity    int
	SortOrder   int
	Reference   *string
	Notes       *string
	// Joined from components
	ComponentMPN          *string
	ComponentManufacturer *string
	ComponentDescription  *string
	ComponentQuantity     int
	ComponentCategoryName string
	ComponentLocationName *string
	// Calculated
	Sufficient bool
	Shortage   int
}

type ProjectBuild struct {
	ID         string
	ProjectID  string
	Multiplier int
	Notes      *string
	BuiltAt    time.Time
}

type DuplicateMPNGroup struct {
	MPN        string
	Components []Component
}

type mergeAttrRow struct {
	Def        AttributeDefinition
	EnumValues []EnumValue
	AttrA      *ComponentAttribute // nil if component A doesn't have this attr
	AttrB      *ComponentAttribute // nil if component B doesn't have this attr
	DisplayA   string              // human-readable value from A
	DisplayB   string              // human-readable value from B
	RawA       string              // form-submission-ready value from A
	RawB       string              // form-submission-ready value from B
	Default    string              // pre-filled final value (A if set, else B)
}

type mergePage struct {
	CompA      *Component
	CompB      *Component
	AttrRows   []mergeAttrRow
	Categories []CategoryListItem
	Locations  []StorageLocation
	SumQty     int
}

type mergeListPage struct {
	Groups []DuplicateMPNGroup
}

type AuditLogEntry struct {
	ID          string
	TableName   string
	RecordID    string
	Action      string
	OldValues   *string // JSON string
	NewValues   *string // JSON string
	ActorUserID *string
	ActorEmail  *string
	CreatedAt   time.Time
}
