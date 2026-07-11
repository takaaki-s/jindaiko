package cmd

import "testing"

func TestActionPopupCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"action-popup"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd == nil || cmd.Name() != "action-popup" {
		t.Fatalf("action-popup not registered: %+v", cmd)
	}
	if !cmd.Hidden {
		t.Errorf("action-popup should be Hidden")
	}
}
