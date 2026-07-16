package config

import "testing"

func TestNormalizeTmuxKey(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"M-p", "M-p"},
		{"C-f", "C-f"},
		{"M-\\", "M-\\"},
		{"q", "q"},
		{"?", "?"},
		{"ctrl+f", "C-f"},
		{"Ctrl+F", "C-F"},
		{"CTRL+f", "C-f"},
		{"alt+n", "M-n"},
		{"Alt+N", "M-N"},
		{"meta+j", "M-j"},
		{"shift+tab", "S-tab"},
		{"ctrl+alt+p", "C-M-p"},
		{"shift+ctrl+alt+p", "S-C-M-p"},
		{"hyper+f", "hyper+f"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := normalizeTmuxKey(tc.in)
			if got != tc.want {
				t.Errorf("normalizeTmuxKey(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeTmuxKeys_NilPassThrough(t *testing.T) {
	if got := normalizeTmuxKeys(nil); got != nil {
		t.Errorf("normalizeTmuxKeys(nil) = %v, want nil", got)
	}
	empty := []string{}
	if got := normalizeTmuxKeys(empty); len(got) != 0 {
		t.Errorf("normalizeTmuxKeys(empty) = %v, want empty slice", got)
	}
}

func TestNormalizeTmuxKeys_MixedNotation(t *testing.T) {
	in := []string{"M-p", "ctrl+f", "alt+n"}
	got := normalizeTmuxKeys(in)
	want := []string{"M-p", "C-f", "M-n"}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
