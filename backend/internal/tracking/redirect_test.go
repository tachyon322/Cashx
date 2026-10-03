package tracking

import "testing"

func TestNormalizeCode(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain code", "LITGWIN", "LITGWIN"},
		{"lowercase", "litgwin", "LITGWIN"},
		{"trailing space from a messenger link", "LITGWIN ", "LITGWIN"},
		{"leading space", " LITGWIN", "LITGWIN"},
		{"non-breaking space", "LITGWIN\u00a0", "LITGWIN"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeCode(tc.in); got != tc.want {
				t.Fatalf("normalizeCode(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
