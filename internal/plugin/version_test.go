package plugin

import (
	"strings"
	"testing"
)

func TestCheckAPIVersion_InWindow(t *testing.T) {
	for v := MinAPIVersion; v <= CurrentAPIVersion; v++ {
		if err := CheckAPIVersion(v); err != nil {
			t.Errorf("CheckAPIVersion(%d) = %v, want nil", v, err)
		}
	}
}

func TestCheckAPIVersion_TooNew(t *testing.T) {
	err := CheckAPIVersion(CurrentAPIVersion + 1)
	if err == nil {
		t.Fatal("CheckAPIVersion above window: want error, got nil")
	}
	if !strings.Contains(err.Error(), "upgrade jin") {
		t.Errorf("error = %q, want it to advise upgrading jin", err)
	}
}

func TestCheckAPIVersion_TooOld(t *testing.T) {
	err := CheckAPIVersion(MinAPIVersion - 1)
	if err == nil {
		t.Fatal("CheckAPIVersion below window: want error, got nil")
	}
	if !strings.Contains(err.Error(), "update the plugin") {
		t.Errorf("error = %q, want it to advise updating the plugin", err)
	}
}
