package convention

import (
	"reflect"
	"testing"

	"github.com/alxxpersonal/stardust/internal/collections"
)

func TestDocCollectionFields(t *testing.T) {
	byName := map[string]DocCollection{}
	for _, c := range DefaultDocCollections() {
		byName[c.Name] = c
	}
	cases := []struct {
		name     string
		docType  string
		statuses []string
	}{
		{"specs", "spec", []string{"Draft", "In Review", "Approved", "Implemented", "Superseded"}},
		{"plans", "plan", []string{"Draft", "Active", "Done", "Abandoned"}},
		{"adr", "adr", []string{"Proposed", "Accepted", "Deferred", "Rejected", "Superseded"}},
		{"research", "research", []string{"Active", "Archived", "Superseded"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, ok := byName[tc.name]
			if !ok {
				t.Fatalf("DefaultDocCollections missing %s", tc.name)
			}
			want := []collections.Field{
				{Name: "title", Type: collections.TypeString, Required: true},
				{Name: "type", Type: collections.TypeEnum, Required: true, Enum: []string{tc.docType}},
				{Name: "status", Type: collections.TypeEnum, Required: true, Enum: tc.statuses},
				{Name: "created", Type: collections.TypeDate, Required: true},
				{Name: "updated", Type: collections.TypeDate, Required: true},
				{Name: "governs", Type: collections.TypeRef, Required: false},
				{Name: "related", Type: collections.TypeRef, Required: false},
			}
			got := c.Fields()
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Fields() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestDocStatusAllowed(t *testing.T) {
	tests := []struct {
		docType string
		status  string
	}{
		{"spec", "Draft"},
		{"spec", "In Review"},
		{"spec", "Approved"},
		{"spec", "Implemented"},
		{"spec", "Superseded"},
		{"plan", "Draft"},
		{"plan", "Active"},
		{"plan", "Done"},
		{"plan", "Abandoned"},
		{"adr", "Proposed"},
		{"adr", "Accepted"},
		{"adr", "Deferred"},
		{"adr", "Rejected"},
		{"adr", "Superseded"},
		{"research", "Active"},
		{"research", "Archived"},
		{"research", "Superseded"},
	}
	for _, tt := range tests {
		t.Run(tt.docType+"/"+tt.status, func(t *testing.T) {
			if !DocStatusAllowed(tt.docType, tt.status) {
				t.Fatalf("DocStatusAllowed(%q, %q) = false, want true", tt.docType, tt.status)
			}
		})
	}
	if DocStatusAllowed("spec", "Weird") {
		t.Fatal("DocStatusAllowed accepted an unknown status")
	}
}

func TestStringList(t *testing.T) {
	got, err := StringList(map[string]any{"governs": []any{"internal/*.go", "cmd/stardust/main.go"}}, "governs")
	if err != nil {
		t.Fatalf("StringList() error = %v", err)
	}
	want := []string{"internal/*.go", "cmd/stardust/main.go"}
	if !sameStrings(got, want) {
		t.Fatalf("StringList() = %#v, want %#v", got, want)
	}

	got, err = StringList(map[string]any{"related": []string{"docs/adr/0001.md"}}, "related")
	if err != nil {
		t.Fatalf("StringList() []string error = %v", err)
	}
	if !sameStrings(got, []string{"docs/adr/0001.md"}) {
		t.Fatalf("StringList() []string = %#v", got)
	}
}

func TestStringListRejectsNonStringItems(t *testing.T) {
	_, err := StringList(map[string]any{"governs": []any{"internal/*.go", 42}}, "governs")
	if err == nil {
		t.Fatal("StringList() error = nil, want non-string item error")
	}
}

func sameStrings(a, b []string) bool {
	return reflect.DeepEqual(a, b)
}
