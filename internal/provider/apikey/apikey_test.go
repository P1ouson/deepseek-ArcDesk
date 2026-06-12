package apikey

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"sk-abc", "sk-abc"},
		{"  sk-abc  ", "sk-abc"},
		{"Bearer sk-abc", "sk-abc"},
		{"bearer sk-abc", "sk-abc"},
		{"BEARER  sk-abc", "sk-abc"},
		{"Bearer Bearer sk-abc", "sk-abc"},
	}
	for _, tc := range tests {
		if got := Normalize(tc.in); got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
