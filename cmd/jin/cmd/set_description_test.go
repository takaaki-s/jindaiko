package cmd

import "testing"

// TestSetDescriptionCmd_NoHostFlag confirms set-description does NOT expose a
// --host flag. HostID is resolved from the selector (see resolveSelector), so
// exposing a redundant flag would let users accidentally target the wrong
// host. Guard against a regression that re-introduces the flag.
func TestSetDescriptionCmd_NoHostFlag(t *testing.T) {
	if flag := setDescriptionCmd.Flags().Lookup("host"); flag != nil {
		t.Error("setDescriptionCmd unexpectedly registered --host flag; HostID must come from the resolved selector")
	}
}

// TestSetDescriptionCmd_ArgsValidation verifies that exactly two positional
// arguments (selector, description) are required.
func TestSetDescriptionCmd_ArgsValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "no args", args: []string{}, wantErr: true},
		{name: "one arg", args: []string{"abcd1234"}, wantErr: true},
		{name: "two args", args: []string{"abcd1234", "new description"}, wantErr: false},
		{name: "two args, empty description", args: []string{"abcd1234", ""}, wantErr: false},
		{name: "three args", args: []string{"abcd1234", "desc", "extra"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setDescriptionCmd.Args(setDescriptionCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

// TestSetDescriptionCmd_LongDescriptionNotEmpty verifies the help text
// documents the unlock-via-empty-string behavior.
func TestSetDescriptionCmd_LongDescriptionNotEmpty(t *testing.T) {
	if setDescriptionCmd.Long == "" {
		t.Error("setDescriptionCmd.Long is empty, want usage examples and unlock explanation")
	}
}
