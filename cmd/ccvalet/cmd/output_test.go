package cmd

import (
	"bytes"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    string
		wantErr bool
	}{
		{
			name:  "struct slice",
			input: []struct{ Name string }{{Name: "alice"}, {Name: "bob"}},
			want: `[
  {
    "Name": "alice"
  },
  {
    "Name": "bob"
  }
]
`,
		},
		{
			name:  "empty slice",
			input: []string{},
			want:  "[]\n",
		},
		{
			name:  "nil input",
			input: nil,
			want:  "null\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := writeJSON(&buf, tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("writeJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("writeJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}
