// Package parser provides SQL dump parsing for offline schema analysis.
package parser

// ColumnDef represents a column definition within a table.
type ColumnDef struct {
	Name          string // Column name
	DataType      string // SQL data type (e.g., "integer", "text")
	NotNull       bool   // Has NOT NULL constraint
	DefaultExpr   string // Default value expression (e.g., "nextval('seq'::regclass)")
	Identity      string // "ALWAYS" or "BY DEFAULT" for GENERATED AS IDENTITY, empty otherwise
	GeneratedExpr string // Expression for GENERATED ALWAYS AS (...) STORED columns
}

// ConstraintDef represents a table constraint (PK, UNIQUE, FK, EXCLUDE, CHECK).
type ConstraintDef struct {
	Name              string   // Constraint name
	ConstraintType    string   // PRIMARY KEY, UNIQUE, FOREIGN KEY, EXCLUDE, CHECK
	TableSchema       string   // Schema containing the table
	TableName         string   // Table name
	Columns           []string // Columns involved in the constraint
	RefSchema         string   // FK: Referenced table schema
	RefTable          string   // FK: Referenced table name
	RefColumns        []string // FK: Referenced columns
	OnDelete          string   // FK: ON DELETE action (CASCADE, SET NULL, etc.)
	OnUpdate          string   // FK: ON UPDATE action
	Deferrable        bool     // Constraint is DEFERRABLE
	InitiallyDeferred bool     // Constraint is INITIALLY DEFERRED
}

// IndexDef represents an index on a table.
type IndexDef struct {
	Name        string   // Index name
	TableSchema string   // Schema containing the table
	TableName   string   // Table name
	Columns     []string // Indexed columns
	IsUnique    bool     // Is a unique index
	Method      string   // Index method (btree, hash, gist, etc.)
}

// SequenceDef represents a sequence object.
type SequenceDef struct {
	SchemaName    string // Schema containing the sequence
	SequenceName  string // Sequence name
	DataType      string // Data type: bigint, integer, smallint
	StartValue    *int64 // START WITH value (nil if not specified)
	Increment     *int64 // INCREMENT BY value (nil if not specified)
	MinValue      *int64 // MINVALUE (nil if not specified)
	MaxValue      *int64 // MAXVALUE (nil if not specified)
	Cycle         bool   // CYCLE option enabled
	OwnedByTable  string // Table owning this sequence (schema.table)
	OwnedByColumn string // Column owning this sequence
}

// TableDef represents a table definition.
type TableDef struct {
	SchemaName  string      // Schema containing the table
	TableName   string      // Table name
	Columns     []ColumnDef // Column definitions
	Unlogged    bool        // Is an UNLOGGED table
	Inherits    []string    // Parent tables (for table inheritance)
	PartitionBy string      // PARTITION BY clause if partitioned
}

// ExtensionDef represents an installed extension.
type ExtensionDef struct {
	Name       string // Extension name
	SchemaName string // Schema where extension is installed
}

// EnumTypeDef represents a custom ENUM type.
type EnumTypeDef struct {
	SchemaName string   // Schema containing the type
	TypeName   string   // Type name
	Labels     []string // Enum labels/values
}

// RuleDef represents a rule on a table.
type RuleDef struct {
	SchemaName string // Schema containing the table
	TableName  string // Table name
	RuleName   string // Rule name
	Event      string // Event: INSERT, UPDATE, DELETE, SELECT
	IsInstead  bool   // Is DO INSTEAD rule
}

// ParsedSchema contains all parsed objects from a pg_dump SQL file.
type ParsedSchema struct {
	Tables      []TableDef      // All tables
	Constraints []ConstraintDef // All constraints
	Indexes     []IndexDef      // All indexes
	Sequences   []SequenceDef   // All sequences
	Extensions  []ExtensionDef  // All extensions
	EnumTypes   []EnumTypeDef   // All ENUM types
	Rules       []RuleDef       // All rules
	PgVersion   string          // PostgreSQL version from dump header
}

// GetTable returns the table definition for the given schema and name, or nil if not found.
func (s *ParsedSchema) GetTable(schema, name string) *TableDef {
	for i := range s.Tables {
		if s.Tables[i].SchemaName == schema && s.Tables[i].TableName == name {
			return &s.Tables[i]
		}
	}
	return nil
}

// GetConstraintsForTable returns all constraints for the given table.
// If constraintType is non-empty, filters to only that type.
func (s *ParsedSchema) GetConstraintsForTable(schema, name, constraintType string) []ConstraintDef {
	var result []ConstraintDef
	for _, c := range s.Constraints {
		if c.TableSchema == schema && c.TableName == name {
			if constraintType == "" || c.ConstraintType == constraintType {
				result = append(result, c)
			}
		}
	}
	return result
}

// GetIndexesForTable returns all indexes for the given table.
func (s *ParsedSchema) GetIndexesForTable(schema, name string) []IndexDef {
	var result []IndexDef
	for _, idx := range s.Indexes {
		if idx.TableSchema == schema && idx.TableName == name {
			result = append(result, idx)
		}
	}
	return result
}
