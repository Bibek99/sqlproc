package sqlproc

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ReturnKind indicates the expected result cardinality.
type ReturnKind string

const (
	// ReturnOne indicates the procedure returns a single row.
	ReturnOne ReturnKind = ":one"
	// ReturnMany indicates the procedure returns multiple rows.
	ReturnMany ReturnKind = ":many"
	// ReturnExec indicates the procedure does not return rows.
	ReturnExec ReturnKind = ":exec"
)

// Procedure represents a parsed stored procedure/function.
type Procedure struct {
	Name    string
	File    string
	SQL     string
	Kind    ReturnKind
	Params  []Param
	Returns []Column
}

// Param describes a single procedure parameter.
type Param struct {
	Name   string
	DBType string
}

// Column describes a column returned by the procedure.
type Column struct {
	Name   string
	DBType string
}

// Parser parses SQL files containing stored procedures.
type Parser struct {
	namePattern    *regexp.Regexp
	paramPattern   *regexp.Regexp
	returnsPattern *regexp.Regexp
}

// NewParser creates a new Parser.
func NewParser() *Parser {
	return &Parser{
		namePattern:    regexp.MustCompile(`--\s*name:\s*([A-Za-z0-9_]+)\s*(:(one|many|exec))`),
		paramPattern:   regexp.MustCompile(`--\s*param:\s*([A-Za-z0-9_]+)\s+(.+)`),
		returnsPattern: regexp.MustCompile(`--\s*returns:\s*(.+)`),
	}
}

// ParseFiles parses a list of SQL files.
func (p *Parser) ParseFiles(files []string) ([]*Procedure, error) {
	var procedures []*Procedure
	for _, file := range files {
		proc, err := p.ParseFile(file)
		if err != nil {
			return nil, err
		}
		procedures = append(procedures, proc)
	}
	return procedures, nil
}

// ParseFile parses a single SQL file and extracts metadata plus SQL body.
func (p *Parser) ParseFile(path string) (*Procedure, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open SQL file: %w", err)
	}
	defer fd.Close()

	proc := &Procedure{
		File:    path,
		Params:  make([]Param, 0),
		Returns: make([]Column, 0),
	}

	scanner := bufio.NewScanner(fd)
	var sqlLines []string
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if matches := p.namePattern.FindStringSubmatch(line); matches != nil {
			proc.Name = matches[1]
			proc.Kind = ReturnKind(matches[2])
			continue
		}

		if matches := p.paramPattern.FindStringSubmatch(line); matches != nil {
			proc.Params = append(proc.Params, Param{
				Name:   matches[1],
				DBType: normalizeType(matches[2]),
			})
			continue
		}

		if matches := p.returnsPattern.FindStringSubmatch(line); matches != nil {
			cols := p.parseColumns(matches[1])
			proc.Returns = append(proc.Returns, cols...)
			continue
		}

		if !strings.HasPrefix(trimmed, "--") {
			sqlLines = append(sqlLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan SQL file: %w", err)
	}

	proc.SQL = strings.TrimSpace(strings.Join(sqlLines, "\n"))

	if err := proc.Validate(); err != nil {
		return nil, fmt.Errorf("invalid procedure %s: %w", path, err)
	}

	return proc, nil
}

func (p *Parser) parseColumns(def string) []Column {
	var columns []Column
	for _, part := range strings.Split(def, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments := strings.Fields(part)
		if len(segments) < 2 {
			continue
		}
		name := segments[0]
		dbType := normalizeType(strings.Join(segments[1:], " "))
		columns = append(columns, Column{Name: name, DBType: dbType})
	}
	return columns
}

// CollectSQLFiles walks a directory and returns all .sql files.
func CollectSQLFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat path: %w", err)
	}

	if !info.IsDir() {
		return []string{root}, nil
	}

	var files []string
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".sql") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk sql dir: %w", err)
	}
	return files, nil
}

// Validate ensures required metadata is present.
func (p *Procedure) Validate() error {
	if p.Name == "" {
		return errors.New("missing -- name metadata")
	}
	switch p.Kind {
	case ReturnOne, ReturnMany, ReturnExec:
	default:
		return fmt.Errorf("unknown return kind %q", p.Kind)
	}
	if p.Kind != ReturnExec && len(p.Returns) == 0 {
		return errors.New("returning procedure must declare -- returns columns")
	}
	if p.SQL == "" {
		return errors.New("procedure SQL body is empty")
	}
	return nil
}

func normalizeType(dbType string) string {
	return strings.TrimSpace(strings.ToLower(dbType))
}
