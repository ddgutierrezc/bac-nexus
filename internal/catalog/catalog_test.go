package catalog

import (
	"errors"
	"strings"
	"testing"
)

func TestIsSystemName(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"letters and digits", "PISA061", true},
		{"dollar prefix", "$SOURCE", true},
		{"dollar inside", "ABC$1", true},
		{"maximum length", "ABCDEFGHIJ", true},
		{"digit prefix", "1SOURCE", false},
		{"too long", "ABCDEFGHIJK", false},
		{"path separator", "LIB/FILE", false},
		{"wildcard", "SRC%", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSystemName(tt.value); got != tt.want {
				t.Fatalf("IsSystemName(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestBuildQueryUsesParametersAndBounds(t *testing.T) {
	query, err := BuildQuery("pisa061", "prod$lib")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(query.Statement, "PISA061") || strings.Contains(query.Statement, "PROD$LIB") {
		t.Fatal("query embeds user input")
	}
	if got, want := query.Parameters, []string{"%PISA061%", "PROD$LIB"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("parameters = %#v, want %#v", got, want)
	}
	if !strings.Contains(query.Statement, "FETCH FIRST 51 ROWS ONLY") {
		t.Fatal("query does not request the cap sentinel row")
	}
}

func TestMemberPath(t *testing.T) {
	candidate := Candidate{Item: "PISA061", SourceLibrary: "SRC$LIB", SourceFileBase: "Q", ObjectType: "RPGLE", SourceType: "RPGLE"}
	got, err := candidate.MemberPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := "SRC$LIB/QRPGLE/PISA061.RPGLE"; got != want {
		t.Fatalf("MemberPath() = %q, want %q", got, want)
	}
}

func TestQSYSPath(t *testing.T) {
	candidate := Candidate{Item: "PISA061", SourceLibrary: "SRC$LIB", SourceFileBase: "Q", ObjectType: "RPGLE", SourceType: "RPGLE"}
	got, err := candidate.QSYSPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := "/QSYS.LIB/SRC$LIB.LIB/QRPGLE.FILE/PISA061.MBR"; got != want {
		t.Fatalf("QSYSPath() = %q, want %q", got, want)
	}
}

func TestBoundedCandidatesRejectsSentinelRow(t *testing.T) {
	_, err := BoundedCandidates(make([]Candidate, MaxCandidates+1))
	if !errors.Is(err, ErrCandidateLimit) {
		t.Fatalf("error = %v, want ErrCandidateLimit", err)
	}
}

func TestSelectDoesNotGuess(t *testing.T) {
	first := Candidate{Item: "ITEM", SourceLibrary: "LIBA", SourceFileBase: "Q", ObjectType: "RPG", SourceType: "RPG"}
	second := first
	second.SourceLibrary = "LIBB"
	selected, err := Select([]Candidate{first, second}, second)
	if err != nil {
		t.Fatal(err)
	}
	if selected.SourceLibrary != "LIBB" {
		t.Fatalf("selected %#v", selected)
	}
}

func TestResolveUniqueReturnsTypedAmbiguity(t *testing.T) {
	rows := []Candidate{{Item: "ITEM"}, {Item: "ITEM"}}
	_, err := ResolveUnique(rows)
	var ambiguous *AmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("error = %v, want AmbiguousError", err)
	}
	if len(ambiguous.Candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(ambiguous.Candidates))
	}
}
