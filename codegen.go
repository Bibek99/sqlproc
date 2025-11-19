package sqlproc

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

// CodeGenerator renders Go files from procedure definitions.
type CodeGenerator struct {
	OutputDir   string
	PackageName string
}

func (cg *CodeGenerator) Generate(procs []*Procedure) error {
	if len(procs) == 0 {
		return nil
	}
	if cg.PackageName == "" {
		cg.PackageName = "generated"
	}

	if err := cg.writeFile("db.go", cg.render(dbTemplate, procs)); err != nil {
		return err
	}
	if err := cg.writeFile("models.go", cg.render(modelsTemplate, procs)); err != nil {
		return err
	}
	if err := cg.writeFile("queries.go", cg.render(queriesTemplate, procs)); err != nil {
		return err
	}
	return nil
}

func (cg *CodeGenerator) render(tmplStr string, procs []*Procedure) []byte {
	tmpl := template.Must(template.New("sqlproc").Funcs(template.FuncMap{
		"GoName":          toGoName,
		"GoField":         toGoExportedField,
		"GoType":          sqlTypeToGo,
		"ReturnKind":      func(p *Procedure, want ReturnKind) bool { return p.Kind == want },
		"ParamSignature":  paramSignature,
		"ArgList":         argList,
		"PlaceholderList": placeholderList,
		"QueryLiteral":    queryLiteral,
		"ScanTargets":     scanTargets,
		"HasParams":       func(p *Procedure) bool { return len(p.Params) > 0 },
	}).Parse(tmplStr))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]any{
		"Package":    cg.PackageName,
		"Procedures": procs,
		"UsesTime":   usesTime(procs),
	}); err != nil {
		panic(err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// return unformatted when formatting fails to ease debugging
		return buf.Bytes()
	}
	return formatted
}

func (cg *CodeGenerator) writeFile(name string, contents []byte) error {
	path := filepath.Join(cg.OutputDir, name)
	return os.WriteFile(path, contents, 0o644)
}

func usesTime(procs []*Procedure) bool {
	for _, proc := range procs {
		for _, col := range proc.Returns {
			if sqlTypeToGo(col.DBType) == "time.Time" {
				return true
			}
		}
	}
	return false
}

func toGoName(name string) string {
	return toCamel(name, true)
}

func toGoExportedField(name string) string {
	return toCamel(name, true)
}

func hasDelimiter(s string) bool {
	for _, r := range s {
		if r == '_' || r == '-' || r == ' ' {
			return true
		}
	}
	return false
}

func toCamel(s string, export bool) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !hasDelimiter(s) {
		if export {
			return strings.ToUpper(s[:1]) + s[1:]
		}
		return strings.ToLower(s[:1]) + s[1:]
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == ' ' || r == '-'
	})
	for i, part := range parts {
		part = strings.ToLower(part)
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	result := strings.Join(parts, "")
	if !export && len(result) > 0 {
		result = strings.ToLower(result[:1]) + result[1:]
	}
	return result
}

func sqlTypeToGo(dbType string) string {
	dbType = strings.ToLower(strings.TrimSpace(dbType))
	switch {
	case strings.HasPrefix(dbType, "int"):
		return "int32"
	case strings.HasPrefix(dbType, "bigint"):
		return "int64"
	case strings.HasPrefix(dbType, "smallint"):
		return "int16"
	case dbType == "serial":
		return "int32"
	case dbType == "bigserial":
		return "int64"
	case strings.Contains(dbType, "char"), strings.Contains(dbType, "text"), strings.Contains(dbType, "uuid"):
		return "string"
	case strings.HasPrefix(dbType, "bool"):
		return "bool"
	case strings.Contains(dbType, "double"), strings.Contains(dbType, "float"), strings.Contains(dbType, "numeric"), strings.Contains(dbType, "decimal"):
		return "float64"
	case strings.Contains(dbType, "timestamp"), strings.Contains(dbType, "date"):
		return "time.Time"
	case strings.Contains(dbType, "json"):
		return "[]byte"
	default:
		return "interface{}"
	}
}

func paramSignature(p *Procedure) string {
	if len(p.Params) == 0 {
		return ""
	}
	var parts []string
	for _, param := range p.Params {
		parts = append(parts, fmt.Sprintf("%s %s", toCamel(param.Name, false), sqlTypeToGo(param.DBType)))
	}
	return ", " + strings.Join(parts, ", ")
}

func argList(p *Procedure) string {
	if len(p.Params) == 0 {
		return ""
	}
	var args []string
	for _, param := range p.Params {
		args = append(args, toCamel(param.Name, false))
	}
	return ", " + strings.Join(args, ", ")
}

func placeholderList(p *Procedure) string {
	if len(p.Params) == 0 {
		return ""
	}
	list := make([]string, len(p.Params))
	for i := range p.Params {
		list[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(list, ", ")
}

func sqlName(p *Procedure) string {
	if p.SQLName != "" {
		return p.SQLName
	}
	return p.Name
}

func callSQL(p *Procedure) string {
	args := placeholderList(p)
	name := sqlName(p)
	if args == "" {
		return fmt.Sprintf("%s()", name)
	}
	return fmt.Sprintf("%s(%s)", name, args)
}

func selectSQL(p *Procedure) string {
	if p.Kind == ReturnExec {
		return "SELECT " + callSQL(p)
	}
	return "SELECT * FROM " + callSQL(p)
}

func queryLiteral(p *Procedure) string {
	sql := selectSQL(p)
	return strconv.Quote(sql)
}

func scanTargets(p *Procedure) string {
	if len(p.Returns) == 0 {
		return ""
	}
	var parts []string
	for _, col := range p.Returns {
		parts = append(parts, "&dest."+toGoExportedField(col.Name))
	}
	return strings.Join(parts, ", ")
}

const dbTemplate = `package {{ .Package }}

import (
	"context"
	"database/sql"
)

type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type Queries struct {
	db DBTX
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

func (q *Queries) WithTx(tx *sql.Tx) *Queries {
	return &Queries{db: tx}
}
`

const modelsTemplate = `package {{ .Package }}

{{- if .UsesTime }}
import "time"
{{- end }}

{{ range .Procedures -}}
{{ if not (ReturnKind . ":exec") -}}
type {{ GoName .Name }}Row struct {
	{{- range .Returns }}
	{{ GoField .Name }} {{ GoType .DBType }}
	{{- end }}
}
{{ end -}}
{{ end }}
`

const queriesTemplate = `package {{ .Package }}

import "context"

{{ range .Procedures -}}
func (q *Queries) {{ GoName .Name }}(ctx context.Context{{ ParamSignature . }}) {{ if ReturnKind . ":exec" }}error{{ else if ReturnKind . ":one" }}({{ GoName .Name }}Row, error){{ else }}([]{{ GoName .Name }}Row, error){{ end }} {
	query := {{ QueryLiteral . }}
	{{ if ReturnKind . ":exec" -}}
	_, err := q.db.ExecContext(ctx, query{{ ArgList . }})
	return err
	{{- else if ReturnKind . ":one" -}}
	row := q.db.QueryRowContext(ctx, query{{ ArgList . }})
	var dest {{ GoName .Name }}Row
	if err := row.Scan({{ ScanTargets . }}); err != nil {
		return dest, err
	}
	return dest, nil
	{{- else -}}
	rows, err := q.db.QueryContext(ctx, query{{ ArgList . }})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]{{ GoName .Name }}Row, 0)
	for rows.Next() {
		var dest {{ GoName .Name }}Row
		if err := rows.Scan({{ ScanTargets . }}); err != nil {
			return nil, err
		}
		result = append(result, dest)
	}
	return result, rows.Err()
	{{- end }}
}

{{ end }}
`
