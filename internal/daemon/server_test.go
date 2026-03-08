package daemon

import (
	"testing"
)

func TestReplaceEnv_Replace(t *testing.T) {
	env := []string{"FOO=old", "BAR=baz"}
	got := replaceEnv(env, "FOO", "new")

	found := false
	for _, e := range got {
		if e == "FOO=new" {
			found = true
		}
		if e == "FOO=old" {
			t.Error("replaceEnv should have replaced FOO=old, but it still exists")
		}
	}
	if !found {
		t.Errorf("replaceEnv did not set FOO=new, got %v", got)
	}
	// BAR should remain unchanged
	barFound := false
	for _, e := range got {
		if e == "BAR=baz" {
			barFound = true
		}
	}
	if !barFound {
		t.Errorf("replaceEnv should preserve other entries, BAR=baz missing in %v", got)
	}
}

func TestReplaceEnv_Append(t *testing.T) {
	env := []string{"BAR=1"}
	got := replaceEnv(env, "FOO", "new")

	if len(got) != 2 {
		t.Fatalf("replaceEnv should append, got len %d: %v", len(got), got)
	}

	// BAR=1 should still be there
	if got[0] != "BAR=1" {
		t.Errorf("first element should be BAR=1, got %q", got[0])
	}
	// FOO=new should be appended
	if got[1] != "FOO=new" {
		t.Errorf("appended element should be FOO=new, got %q", got[1])
	}
}

func TestReplaceEnv_Empty(t *testing.T) {
	var env []string
	got := replaceEnv(env, "FOO", "bar")

	if len(got) != 1 {
		t.Fatalf("replaceEnv on empty env should return 1 element, got %d: %v", len(got), got)
	}
	if got[0] != "FOO=bar" {
		t.Errorf("replaceEnv on empty env should return [FOO=bar], got %v", got)
	}
}

func TestReplaceEnv_PrefixCollision(t *testing.T) {
	// Ensure "FOOBAR=x" is not matched when replacing "FOO"
	env := []string{"FOOBAR=x", "FOO=old"}
	got := replaceEnv(env, "FOO", "new")

	foobarFound := false
	fooNewFound := false
	for _, e := range got {
		if e == "FOOBAR=x" {
			foobarFound = true
		}
		if e == "FOO=new" {
			fooNewFound = true
		}
	}
	if !foobarFound {
		t.Errorf("FOOBAR=x should not be modified, got %v", got)
	}
	if !fooNewFound {
		t.Errorf("FOO=new should be present, got %v", got)
	}
}

