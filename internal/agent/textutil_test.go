package agent

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSmartTruncate(t *testing.T) {
	const max = 60

	// A 70-byte ASCII string with word boundaries at 20 and 50 bytes.
	long := "abcdefghij klmnopqrst uvwxyzABCD EFGHIJKLMN OPQRSTUVWX YZ0123456789abcd"

	// 70-byte ASCII string with no whitespace at all — forces hard cut.
	dense := strings.Repeat("x", 70)

	// Long UTF-8 string with no word boundaries: 21 copies of "あ" = 63 bytes.
	// A naive byte cut at 60 would land inside the 20th rune (bytes 57..59 form
	// rune 20; bytes 60..62 form rune 21). Cutting at byte 60 is exactly at a
	// rune boundary, so the utf8 back-off must not fire. Add an extra byte at
	// the end to guarantee the input exceeds the budget.
	jp := strings.Repeat("あ", 21) // 63 bytes

	tests := []struct {
		name       string
		in         string
		wantExact  string // if non-empty, exact match required
		wantMaxLen int    // otherwise, len(result) <= wantMaxLen (bytes, including ellipsis)
		wantSuffix string // optional suffix check (e.g. "…")
	}{
		{
			name:      "short input is returned unchanged",
			in:        "hello",
			wantExact: "hello",
		},
		{
			name:      "exactly maxBytes is returned unchanged",
			in:        strings.Repeat("a", max),
			wantExact: strings.Repeat("a", max),
		},
		{
			name:      "newline splits and takes the first line",
			in:        "first line\nsecond line goes here too",
			wantExact: "first line",
		},
		{
			name:      "whitespace only after newline strip yields empty",
			in:        "\n\n\n",
			wantExact: "",
		},
		{
			name:       "long ASCII with boundary cuts at whitespace",
			in:         long,
			wantMaxLen: max + len("…"),
			wantSuffix: "…",
		},
		{
			name:       "no whitespace performs hard cut",
			in:         dense,
			wantMaxLen: max + len("…"),
			wantSuffix: "…",
		},
		{
			name:       "utf8 hard cut backs off to a rune boundary",
			in:         jp,
			wantMaxLen: max + len("…"),
			wantSuffix: "…",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SmartTruncate(tc.in, max)
			if tc.wantExact != "" || tc.wantMaxLen == 0 {
				if got != tc.wantExact {
					t.Errorf("got = %q, want %q", got, tc.wantExact)
				}
				return
			}
			if len(got) > tc.wantMaxLen {
				t.Errorf("len(got) = %d, want <= %d (got=%q)", len(got), tc.wantMaxLen, got)
			}
			if tc.wantSuffix != "" && !strings.HasSuffix(got, tc.wantSuffix) {
				t.Errorf("got = %q, want suffix %q", got, tc.wantSuffix)
			}
			if !utf8.ValidString(got) {
				t.Errorf("got = %q is not valid UTF-8", got)
			}
		})
	}
}
