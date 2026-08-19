package source

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func newSnapshot(t *testing.T, content string) *Snapshot {
	t.Helper()
	snapshot, err := NewSnapshot([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestSnapshotPagesCompleteUTF8Lines(t *testing.T) {
	snapshot := newSnapshot(t, "first\nnaïve  \n最後")

	page, err := snapshot.Page(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := page.Lines, []string{"naïve  ", "最後"}; !equalLines(got, want) {
		t.Fatalf("lines = %#v, want %#v", got, want)
	}
	if page.StartLine != 2 || page.LineCount != 2 || !page.EOF || page.NextStartLine != 0 {
		t.Fatalf("page = %#v", page)
	}
}

func TestSnapshotRecognizesEmptyAndFinalRecords(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty member", input: "", want: nil},
		{name: "final record without LF", input: "final", want: []string{"final"}},
		{name: "LF does not add a record", input: "one\n", want: []string{"one"}},
		{name: "trailing spaces preserved", input: "ab  \ncd  ", want: []string{"ab  ", "cd  "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := newSnapshot(t, tt.input)
			page, err := snapshot.Page(1, 10)
			if err != nil {
				t.Fatal(err)
			}
			if !equalLines(page.Lines, tt.want) || !page.EOF || page.NextStartLine != 0 {
				t.Fatalf("page = %#v, want lines %#v and EOF", page, tt.want)
			}
		})
	}
}

func TestSnapshotRejectsOversizedPageLimits(t *testing.T) {
	// A request above the 200-line cap MUST be rejected, never silently clamped.
	lines := make([]string, MaxPageLines+1)
	for i := range lines {
		lines[i] = "x"
	}
	snapshot := newSnapshot(t, strings.Join(lines, "\n"))
	page, err := snapshot.Page(1, MaxPageLines+1)
	if !errors.Is(err, ErrInvalidRequest) || page.LineCount != 0 || len(page.Lines) != 0 || page.NextStartLine != 0 {
		t.Fatalf("maxLines > cap result = %#v, %v", page, err)
	}

	// A first line whose marshaled envelope already exceeds the 128 KiB cap
	// MUST yield ErrResponseTooLarge with no partial content.
	snapshot = newSnapshot(t, strings.Repeat("界", MaxPageBytes/3)+"\nnext")
	page, err = snapshot.Page(1, 2)
	if !errors.Is(err, ErrResponseTooLarge) || page.LineCount != 0 || len(page.Lines) != 0 || page.NextStartLine != 0 {
		t.Fatalf("oversized line result = %#v, %v", page, err)
	}
}

func TestSnapshotRejectsInvalidRangesWithoutPartialContent(t *testing.T) {
	snapshot := newSnapshot(t, "one\ntwo")
	for _, tt := range []struct {
		name  string
		start int
		max   int
		want  error
	}{
		{name: "zero start", start: 0, max: 1, want: ErrInvalidRequest},
		{name: "zero max", start: 1, max: 0, want: ErrInvalidRequest},
		{name: "max above cap", start: 1, max: MaxPageLines + 1, want: ErrInvalidRequest},
		{name: "past final line", start: 3, max: 1, want: ErrRangeStartOutOfBounds},
	} {
		t.Run(tt.name, func(t *testing.T) {
			page, err := snapshot.Page(tt.start, tt.max)
			if !errors.Is(err, tt.want) || page.LineCount != 0 || len(page.Lines) != 0 || page.NextStartLine != 0 {
				t.Fatalf("page = %#v, error = %v", page, err)
			}
		})
	}
}

func TestSnapshotPageStaysWithinMarshaledLimit(t *testing.T) {
	snapshot := newSnapshot(t, strings.Repeat("ab\n", 100))
	page, err := snapshot.Page(1, 100)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > MaxPageBytes {
		t.Fatalf("marshaled page size = %d, limit = %d", len(encoded), MaxPageBytes)
	}
}

func TestSnapshotCopiesContentImmutable(t *testing.T) {
	// Verifies NewSnapshot does not alias the caller's slice. A string-based
	// helper would hide the buffer from the test, so construct directly.
	buffer := []byte("alpha\nbeta")
	original := append([]byte(nil), buffer...)
	snapshot, err := NewSnapshot(buffer)
	if err != nil {
		t.Fatal(err)
	}
	buffer[0] = 'X'
	page, err := snapshot.Page(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !equalLines(page.Lines, []string{"alpha", "beta"}) {
		t.Fatalf("lines = %#v; original input was %q", page.Lines, original)
	}
}

func equalLines(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
