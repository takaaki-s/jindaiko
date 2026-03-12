package cmd

import (
	"encoding/json"
	"io"
	"os"
)

// writeJSON encodes v as indented JSON to w.
// Note: nil slices are encoded as "null"; callers should normalize to empty
// slices before calling if "[]" output is required.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printJSON(v any) error {
	return writeJSON(os.Stdout, v)
}
