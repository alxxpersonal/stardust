// Package collections models structured views over the vault: a collection is a
// vault folder paired with a typed schema declared under
// .stardust/collections/<name>/config.toml. A record is a markdown note in that
// folder; its frontmatter holds the typed columns and its body holds content.
// This package only loads and validates schemas; it never writes notes.
package collections

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// ErrValidation tags a record-frontmatter validation failure. It marks a domain
// error (the request was well formed but its data violates the schema) as
// distinct from an infrastructure failure, so a transport can map it to the
// positive JSON-RPC domain code band rather than the reserved server band (ADR
// 0006). Every non-nil Validate result wraps it; classify with errors.Is.
var ErrValidation = errors.New("validation")

// FieldType enumerates the supported schema field types.
const (
	TypeString = "string"
	TypeNumber = "number"
	TypeBool   = "bool"
	TypeDate   = "date"
	TypeEnum   = "enum"
	TypeTags   = "tags"
	TypeRef    = "ref"
)

// validTypes is the set of field types a schema may declare.
var validTypes = map[string]bool{
	TypeString: true,
	TypeNumber: true,
	TypeBool:   true,
	TypeDate:   true,
	TypeEnum:   true,
	TypeTags:   true,
	TypeRef:    true,
}

// Field is one typed column of a collection schema. Type is one of string,
// number, bool, date, enum, tags, or ref. Enum lists the allowed values for an
// enum field. Default supplies a value when a record omits the field.
type Field struct {
	Name     string   `toml:"name" json:"name"`
	Type     string   `toml:"type" json:"type"`
	Required bool     `toml:"required" json:"required"`
	Enum     []string `toml:"enum" json:"enum,omitempty"`
	Default  any      `toml:"default" json:"default,omitempty"`
}

// Config is the committed schema of a collection
// (.stardust/collections/<name>/config.toml). Path is the vault-relative folder
// the records live in. Fields are the typed frontmatter columns.
type Config struct {
	Path        string  `toml:"path"`
	Description string  `toml:"description"`
	Fields      []Field `toml:"fields"`
}

// Collection is a loaded collection: its directory name and parsed schema.
type Collection struct {
	Name string
	Cfg  Config
}

// Load reads every collection folder under collectionsDir, parsing each
// config.toml into a Collection. A missing dir yields no collections rather than
// an error. Results are sorted by name. Each collection's Path must be non-empty.
func Load(collectionsDir string) ([]Collection, error) {
	entries, err := os.ReadDir(collectionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read collections dir: %w", err)
	}
	var out []Collection
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := LoadOne(collectionsDir, e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// LoadOne reads and validates a single collection by folder name. The Path field
// is required and every field's Type must be one of the supported types.
func LoadOne(collectionsDir, name string) (Collection, error) {
	b, err := os.ReadFile(filepath.Join(collectionsDir, name, "config.toml"))
	if err != nil {
		return Collection{}, fmt.Errorf("read collection %s: %w", name, err)
	}
	var cfg Config
	if err := toml.Unmarshal(b, &cfg); err != nil {
		return Collection{}, fmt.Errorf("parse collection %s: %w", name, err)
	}
	if strings.TrimSpace(cfg.Path) == "" {
		return Collection{}, fmt.Errorf("collection %s: path is required", name)
	}
	for _, f := range cfg.Fields {
		if strings.TrimSpace(f.Name) == "" {
			return Collection{}, fmt.Errorf("collection %s: field with empty name", name)
		}
		if !validTypes[f.Type] {
			return Collection{}, fmt.Errorf("collection %s: field %s has unsupported type %q", name, f.Name, f.Type)
		}
	}
	return Collection{Name: name, Cfg: cfg}, nil
}

// Validate checks a record's frontmatter against a schema's fields: every
// required field must be present and non-nil, every enum field's value must be a
// member of its Enum set, and present values must satisfy a basic type check for
// their declared type. Fields not in the schema are ignored.
func Validate(frontmatter map[string]any, fields []Field) (err error) {
	// Tag every non-nil result with ErrValidation so callers can classify a
	// schema violation as a domain error without losing the specific message.
	defer func() {
		if err != nil && !errors.Is(err, ErrValidation) {
			err = fmt.Errorf("%w: %w", ErrValidation, err)
		}
	}()
	for _, f := range fields {
		v, present := frontmatter[f.Name]
		if !present || v == nil {
			if f.Required {
				return fmt.Errorf("field %s is required", f.Name)
			}
			continue
		}
		if err := checkType(f, v); err != nil {
			return err
		}
		if f.Type == TypeEnum {
			if err := checkEnum(f, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// --- Helpers ---

// checkType verifies that v is a plausible value for the field's declared type.
func checkType(f Field, v any) error {
	switch f.Type {
	case TypeString:
		if _, ok := v.(string); !ok {
			return fmt.Errorf("field %s must be a string", f.Name)
		}
	case TypeRef:
		if err := checkRef(f, v); err != nil {
			return err
		}
	case TypeEnum:
		if _, ok := v.(string); !ok {
			return fmt.Errorf("field %s must be a string", f.Name)
		}
	case TypeNumber:
		if !isNumber(v) {
			return fmt.Errorf("field %s must be a number", f.Name)
		}
	case TypeBool:
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("field %s must be a bool", f.Name)
		}
	case TypeDate:
		if err := checkDate(f, v); err != nil {
			return err
		}
	case TypeTags:
		if err := checkTags(f, v); err != nil {
			return err
		}
	}
	return nil
}

// checkRef verifies that v is a reference: a single target string or a list of
// target strings, matching how related and governs surface from YAML
// frontmatter (commonly a list, occasionally a lone scalar).
func checkRef(f Field, v any) error {
	switch list := v.(type) {
	case string:
		return nil
	case []any:
		for _, item := range list {
			if _, ok := item.(string); !ok {
				return fmt.Errorf("field %s must be a string or list of strings", f.Name)
			}
		}
		return nil
	case []string:
		return nil
	default:
		return fmt.Errorf("field %s must be a string or list of strings", f.Name)
	}
}

// checkEnum verifies that v (a string) is a member of the field's Enum set.
func checkEnum(f Field, v any) error {
	s, _ := v.(string)
	for _, allowed := range f.Enum {
		if s == allowed {
			return nil
		}
	}
	return fmt.Errorf("field %s value %q is not one of %v", f.Name, s, f.Enum)
}

// checkDate verifies that v is a string in YYYY-MM-DD form or a time.Time, which
// is how TOML and YAML surface dates.
func checkDate(f Field, v any) error {
	switch t := v.(type) {
	case time.Time:
		return nil
	case string:
		if _, err := time.Parse("2006-01-02", t); err != nil {
			return fmt.Errorf("field %s must be a YYYY-MM-DD date", f.Name)
		}
		return nil
	default:
		return fmt.Errorf("field %s must be a date", f.Name)
	}
}

// checkTags verifies that v is a list (each element a string) or a single
// string, matching how tags surface from YAML frontmatter.
func checkTags(f Field, v any) error {
	switch list := v.(type) {
	case string:
		return nil
	case []any:
		for _, item := range list {
			if _, ok := item.(string); !ok {
				return fmt.Errorf("field %s must be a list of strings", f.Name)
			}
		}
		return nil
	case []string:
		return nil
	default:
		return fmt.Errorf("field %s must be tags (a string or list of strings)", f.Name)
	}
}

// isNumber reports whether v is one of the numeric kinds that JSON or YAML
// decoding can produce.
func isNumber(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, float32, float64:
		return true
	default:
		return false
	}
}
