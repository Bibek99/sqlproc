package sqlproc

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// SchemaModelOptions configure schema introspection and model generation.
type SchemaModelOptions struct {
	// Schemas limits introspection to the provided schema names. Empty => all user schemas.
	Schemas []string
	// OutputDir is where schema-driven Go files are written.
	OutputDir string
	// PackageName is the Go package name used inside generated schema files.
	PackageName string
	// StructTag defines struct tag keys, comma-separated (e.g. "db,json").
	StructTag string
}

func (o SchemaModelOptions) withDefaults(fallbackDir, fallbackPkg string) SchemaModelOptions {
	if o.OutputDir == "" {
		o.OutputDir = fallbackDir
	}
	if o.PackageName == "" {
		o.PackageName = fallbackPkg
	}
	if o.PackageName == "" {
		o.PackageName = "generated"
	}
	if o.StructTag == "" {
		o.StructTag = "db,json"
	}
	return o
}

// Table represents a database table discovered via introspection.
type Table struct {
	Schema  string
	Name    string
	Columns []TableColumn
}

// TableColumn represents a column in a table.
type TableColumn struct {
	Name     string
	DBType   string
	Nullable bool
}

func loadSchemaTables(ctx context.Context, db *sql.DB, opts SchemaModelOptions) ([]*Table, error) {
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
SELECT table_schema, table_name, column_name, data_type, udt_name, is_nullable
FROM information_schema.columns
WHERE table_schema NOT IN ('pg_catalog', 'information_schema')`)

	var args []any
	if len(opts.Schemas) > 0 {
		queryBuilder.WriteString(" AND table_schema IN (")
		for i, schema := range opts.Schemas {
			if i > 0 {
				queryBuilder.WriteString(", ")
			}
			queryBuilder.WriteString(fmt.Sprintf("$%d", i+1))
			args = append(args, schema)
		}
		queryBuilder.WriteString(")")
	}
	queryBuilder.WriteString(" ORDER BY table_schema, table_name, ordinal_position")

	rows, err := db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rawColumn struct {
		schema    string
		table     string
		column    string
		dataType  string
		udtName   string
		isNullStr string
	}

	var rawCols []rawColumn
	for rows.Next() {
		var rc rawColumn
		if err := rows.Scan(&rc.schema, &rc.table, &rc.column, &rc.dataType, &rc.udtName, &rc.isNullStr); err != nil {
			return nil, err
		}
		rawCols = append(rawCols, rc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tableMap := make(map[string]*Table)
	for _, rc := range rawCols {
		key := rc.schema + "." + rc.table
		table := tableMap[key]
		if table == nil {
			table = &Table{
				Schema:  rc.schema,
				Name:    rc.table,
				Columns: make([]TableColumn, 0),
			}
			tableMap[key] = table
		}
		table.Columns = append(table.Columns, TableColumn{
			Name:     rc.column,
			DBType:   pickDBType(rc.dataType, rc.udtName),
			Nullable: strings.EqualFold(rc.isNullStr, "YES"),
		})
	}

	tables := make([]*Table, 0, len(tableMap))
	for _, table := range tableMap {
		tables = append(tables, table)
	}
	sort.Slice(tables, func(i, j int) bool {
		if tables[i].Schema == tables[j].Schema {
			return tables[i].Name < tables[j].Name
		}
		return tables[i].Schema < tables[j].Schema
	})

	for _, table := range tables {
		sort.Slice(table.Columns, func(i, j int) bool {
			return table.Columns[i].Name < table.Columns[j].Name
		})
	}

	return tables, nil
}

func pickDBType(dataType, udtName string) string {
	switch {
	case udtName != "" && !strings.EqualFold(udtName, "null"):
		return normalizeType(udtName)
	case dataType != "":
		return normalizeType(dataType)
	default:
		return "text"
	}
}

// SchemaModelGenerator renders Go structs for database tables.
type SchemaModelGenerator struct {
	Options SchemaModelOptions
}

// Generate writes schema-based Go models for the provided tables.
func (g *SchemaModelGenerator) Generate(tables []*Table) ([]string, error) {
	if len(tables) == 0 {
		return nil, nil
	}
	if g.Options.OutputDir == "" {
		return nil, errors.New("schema model output directory is required")
	}
	if err := os.MkdirAll(g.Options.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create schema output dir: %w", err)
	}

	data := buildSchemaTemplateData(tables, g.Options.PackageName, g.Options.StructTag)
	var buf bytes.Buffer
	tmpl := template.Must(template.New("schema-models").Parse(schemaModelsTemplate))
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(g.Options.OutputDir, "schema_models.go")
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return nil, fmt.Errorf("write schema models: %w", err)
	}
	return []string{path}, nil
}

type schemaTemplateData struct {
	Package  string
	Tables   []schemaTemplateTable
	UsesTime bool
}

type schemaTemplateTable struct {
	Name    string
	Comment string
	Columns []schemaTemplateColumn
}

type schemaTemplateColumn struct {
	Field string
	Type  string
	Tag   string
}

func buildSchemaTemplateData(tables []*Table, pkg, structTag string) schemaTemplateData {
	if pkg == "" {
		pkg = "generated"
	}
	tagKeys := parseTagKeys(structTag)

	result := schemaTemplateData{
		Package: pkg,
		Tables:  make([]schemaTemplateTable, 0, len(tables)),
	}

	for _, table := range tables {
		tmplTable := schemaTemplateTable{
			Name:    goStructName(table.Schema, table.Name),
			Columns: make([]schemaTemplateColumn, 0, len(table.Columns)),
		}
		for _, col := range table.Columns {
			goType := goTypeForColumn(col)
			tmplCol := schemaTemplateColumn{
				Field: toGoExportedField(col.Name),
				Type:  goType,
				Tag:   buildStructTag(tagKeys, col.Name),
			}
			if strings.Contains(goType, "time.Time") {
				result.UsesTime = true
			}
			tmplTable.Columns = append(tmplTable.Columns, tmplCol)
		}
		result.Tables = append(result.Tables, tmplTable)
	}

	return result
}

func parseTagKeys(spec string) []string {
	if spec == "" {
		return nil
	}
	parts := strings.Split(spec, ",")
	var keys []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			keys = append(keys, part)
		}
	}
	return keys
}

func buildStructTag(keys []string, column string) string {
	if len(keys) == 0 {
		return ""
	}
	var tagParts []string
	for _, key := range keys {
		value := column
		if key == "json" {
			value = toCamel(column, false)
		}
		tagParts = append(tagParts, fmt.Sprintf(`%s:"%s"`, key, value))
	}
	return fmt.Sprintf("`%s`", strings.Join(tagParts, " "))
}

func goStructName(schema, table string) string {
	if schema == "" || schema == "public" {
		return toGoName(table)
	}
	return toGoName(schema + "_" + table)
}

func goTypeForColumn(col TableColumn) string {
	base := sqlTypeToGo(col.DBType)
	if !col.Nullable {
		return base
	}
	if strings.HasPrefix(base, "[]") {
		return base
	}
	return "*" + base
}

const schemaModelsTemplate = `package {{ .Package }}

{{- if .UsesTime }}
import "time"
{{- end }}

{{ range .Tables }}
type {{ .Name }} struct {
{{- range .Columns }}
	{{ .Field }} {{ .Type }}{{ if .Tag }} {{ .Tag }}{{ end }}
{{- end }}
}

{{ end }}
`
