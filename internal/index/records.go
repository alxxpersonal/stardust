package index

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Predicate is a single frontmatter filter applied to ListRecords. Field is a
// top-level frontmatter key, Op is one of eq, ne, gt, gte, lt, lte, contains,
// and Value is the comparison operand (compared as text via json_extract).
type Predicate struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

// RecordRow is one catalog row projected for the records layer: the note path,
// its title, its decoded frontmatter, and the last-indexed timestamp.
type RecordRow struct {
	Path        string         `json:"path"`
	Title       string         `json:"title"`
	Frontmatter map[string]any `json:"frontmatter"`
	UpdatedAt   string         `json:"updated_at"`
}

// predicateSQL maps a Predicate op to the SQL operator applied against the
// json_extract of the field. The contains op is handled separately via LIKE.
var predicateSQL = map[string]string{
	"eq":  "=",
	"ne":  "!=",
	"gt":  ">",
	"gte": ">=",
	"lt":  "<",
	"lte": "<=",
}

// ListRecords returns the catalog rows whose path is under folder (a slash-
// separated prefix), filtered by preds against their JSON frontmatter and
// ordered by sort. preds combine with AND; each compares
// json_extract(frontmatter, '$.'||field) to the predicate value. sort is a
// frontmatter field, or one of "path" / "updated_at", with an optional leading
// "-" for descending order; an empty sort falls back to path ascending. A
// non-positive limit means no limit; offset is applied after ordering.
func (s *Store) ListRecords(ctx context.Context, folder string, preds []Predicate, sort string, limit, offset int) ([]RecordRow, error) {
	var where []string
	var args []any

	if prefix := normalizeFolder(folder); prefix != "" {
		where = append(where, "path LIKE ? ESCAPE '\\'")
		args = append(args, likePrefix(prefix)+"%")
	}

	for _, p := range preds {
		if p.Field == "" {
			return nil, fmt.Errorf("list records: predicate with empty field")
		}
		expr := "json_extract(frontmatter, '$.' || ?)"
		if p.Op == "contains" {
			where = append(where, expr+" LIKE ? ESCAPE '\\'")
			args = append(args, p.Field, "%"+likeEscape(p.Value)+"%")
			continue
		}
		op, ok := predicateSQL[p.Op]
		if !ok {
			return nil, fmt.Errorf("list records: unsupported op %q", p.Op)
		}
		// json_extract returns a numeric value for JSON numbers, but the bound
		// operand is always text; SQLite does not coerce numeric-vs-text, so a
		// numeric value never matches a text literal. When the predicate value
		// parses as a number, cast both sides to REAL for a numeric comparison.
		if _, err := strconv.ParseFloat(p.Value, 64); err == nil {
			where = append(where, "CAST("+expr+" AS REAL) "+op+" CAST(? AS REAL)")
		} else {
			where = append(where, expr+" "+op+" ?")
		}
		args = append(args, p.Field, p.Value)
	}

	q := "SELECT path, title, frontmatter, updated_at FROM catalog"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	// ORDER BY cannot be parameterised in SQLite. orderBy emits a fixed column
	// for the path/updated_at keys and otherwise single-quote escapes the field
	// name into a json_extract literal, so no caller input reaches the SQL
	// unescaped. The WHERE values above are all bound parameters.
	q += " ORDER BY " + orderBy(sort) //nolint:gosec // G202: orderBy output is escaped/whitelisted, not raw input
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			q += " OFFSET ?"
			args = append(args, offset)
		}
	} else if offset > 0 {
		q += " LIMIT -1 OFFSET ?"
		args = append(args, offset)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list records: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []RecordRow
	for rows.Next() {
		var r RecordRow
		var fmJSON string
		if err := rows.Scan(&r.Path, &r.Title, &fmJSON, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan record row: %w", err)
		}
		r.Frontmatter = map[string]any{}
		if fmJSON != "" {
			if err := json.Unmarshal([]byte(fmJSON), &r.Frontmatter); err != nil {
				return nil, fmt.Errorf("decode frontmatter for %s: %w", r.Path, err)
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Helpers ---

// orderBy builds a safe ORDER BY clause from sort. A leading "-" requests
// descending order. The reserved keys "path" and "updated_at" order by those
// columns; any other key orders by its json_extract'd frontmatter field. The
// field name is bound as a literal here (not a parameter) because ORDER BY
// cannot be parameterised, so it is single-quote escaped to stay injection-safe.
func orderBy(sort string) string {
	dir := "ASC"
	field := strings.TrimSpace(sort)
	if strings.HasPrefix(field, "-") {
		dir = "DESC"
		field = strings.TrimSpace(field[1:])
	}
	switch field {
	case "":
		return "path ASC"
	case "path", "updated_at":
		return field + " " + dir
	default:
		return "json_extract(frontmatter, '$." + sqlLiteralEscape(field) + "') " + dir
	}
}

// normalizeFolder trims surrounding slashes from a folder prefix.
func normalizeFolder(folder string) string {
	return strings.Trim(strings.TrimSpace(folder), "/")
}

// likePrefix turns a folder prefix into a LIKE pattern stem, escaping LIKE
// metacharacters and appending a trailing slash so only descendants match.
func likePrefix(folder string) string {
	return likeEscape(folder) + "/"
}

// likeEscape escapes the LIKE metacharacters (%, _, and the escape char) so a
// user value matches literally under "ESCAPE '\\'".
func likeEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// sqlLiteralEscape doubles single quotes so a value is safe inside a single
// quoted SQL string literal.
func sqlLiteralEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
