package convention

import "testing"

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
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
