package mapepiredirect

import "testing"

func TestValidateReuse(t *testing.T) {
	for _, tt := range []struct {
		name string
		jobs []string
		ok   bool
	}{
		{"two jobs reused", []string{"a", "b", "a", "b"}, true},
		{"third job", []string{"a", "b", "a", "c"}, false},
		{"one job", []string{"a", "a", "a", "a"}, false},
		{"empty job", []string{"", "b", "", "b"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := validateReuse(tt.jobs)
			if (err == nil) != tt.ok {
				t.Fatal("unexpected reuse validation")
			}
		})
	}
}
