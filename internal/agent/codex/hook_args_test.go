package codex

import (
	"strings"
	"testing"
)

func TestHookArgs_Empty(t *testing.T) {
	if got := HookArgs(""); got != nil {
		t.Errorf("HookArgs(\"\") = %#v, want nil", got)
	}
}

func TestHookArgs_EnableFlagFirst(t *testing.T) {
	args := HookArgs("/usr/local/bin/jin")
	if len(args) < 2 || args[0] != "--enable" || args[1] != "hooks" {
		t.Errorf("first two args = %v, want [--enable hooks]", args[:min(2, len(args))])
	}
}

func TestHookArgs_AllEventsPresent(t *testing.T) {
	args := HookArgs("/usr/local/bin/jin")
	joined := strings.Join(args, " ")
	for _, ev := range managedEvents {
		if !strings.Contains(joined, "hooks."+ev+"=") {
			t.Errorf("event %q missing from output:\n%s", ev, joined)
		}
	}
}

func TestHookArgs_LengthMatchesManagedEvents(t *testing.T) {
	args := HookArgs("/usr/local/bin/jin")
	// 2 for --enable hooks + 2 per event (-c and the TOML value) = 2 + 2*N
	want := 2 + 2*len(managedEvents)
	if len(args) != want {
		t.Errorf("len(args) = %d, want %d — args=%v", len(args), want, args)
	}
}

func TestHookArgs_Golden(t *testing.T) {
	// The exact shape spawn.go will splice into SpawnPlan.Command. If this
	// ever changes, review the shell-escape trace in the TOML doc comment
	// on tomlEscapeForShell — the outer manager wrapping depends on
	// balanced ' quotes here.
	args := HookArgs("/usr/local/bin/jin")
	got := strings.Join(args, " ")
	want := "--enable hooks" +
		` -c 'hooks.SessionStart=[{hooks=[{type="command",command="/usr/local/bin/jin hook",timeout=10000}]}]'` +
		` -c 'hooks.UserPromptSubmit=[{hooks=[{type="command",command="/usr/local/bin/jin hook",timeout=10000}]}]'` +
		` -c 'hooks.PreToolUse=[{hooks=[{type="command",command="/usr/local/bin/jin hook",timeout=10000}]}]'` +
		` -c 'hooks.PostToolUse=[{hooks=[{type="command",command="/usr/local/bin/jin hook",timeout=10000}]}]'` +
		` -c 'hooks.PermissionRequest=[{hooks=[{type="command",command="/usr/local/bin/jin hook",timeout=10000}]}]'` +
		` -c 'hooks.Stop=[{hooks=[{type="command",command="/usr/local/bin/jin hook",timeout=10000}]}]'`
	if got != want {
		t.Errorf("HookArgs joined mismatch:\nwant: %s\n got: %s", want, got)
	}
}

func TestHookArgs_PathWithSpace(t *testing.T) {
	// Space inside the path is fine because the outer shell single quotes
	// group the whole -c value; Codex's TOML parser then reads
	// command="..." verbatim.
	args := HookArgs("/tmp/dir with space/jin")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, `command="/tmp/dir with space/jin hook"`) {
		t.Errorf("path with space not embedded verbatim:\n%s", joined)
	}
	if strings.Contains(joined, `'`) && !strings.Contains(joined, `-c '`) {
		t.Errorf("expected outer -c '...' grouping to survive space:\n%s", joined)
	}
}

func TestHookArgs_PathWithSingleQuote(t *testing.T) {
	// A ' in the path would break the outer shell -c '...' grouping unless
	// we replace it with a TOML unicode escape. Manager's outer wrap does
	// its own ' → '\'' escape; feeding it a raw ' here would produce
	// unbalanced quoting. The parser inside Codex sees the escape as ' and
	// invokes the correct path.
	args := HookArgs("/tmp/foo'bar/jin")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "command=\"/tmp/foo\\u0027bar/jin hook\"") {
		t.Errorf("single quote not encoded as \\u0027:\n%s", joined)
	}
	// A raw ' must never leak into the emitted TOML value (aside from the
	// outer shell -c '...' grouping quotes). Count both.
	rawSingleQuotes := strings.Count(joined, "'")
	// Expected: 2 outer quotes per -c pair (12 total for 6 events). No extra.
	wantRaw := 2 * len(managedEvents)
	if rawSingleQuotes != wantRaw {
		t.Errorf("raw single quotes in output = %d, want %d (2 per -c grouping):\n%s",
			rawSingleQuotes, wantRaw, joined)
	}
}

func TestHookArgs_PathWithDoubleQuote(t *testing.T) {
	args := HookArgs(`/tmp/quote"path/jin`)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, `command="/tmp/quote\"path/jin hook"`) {
		t.Errorf("double quote not TOML-escaped:\n%s", joined)
	}
}

func TestHookArgs_PathWithBackslash(t *testing.T) {
	args := HookArgs(`C:\Program Files\jin.exe`)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, `command="C:\\Program Files\\jin.exe hook"`) {
		t.Errorf("backslash not TOML-escaped:\n%s", joined)
	}
}

func TestTomlEscapeForShell_NoOpForCleanPath(t *testing.T) {
	// Fast-path optimisation: paths without ' " \ should be returned
	// verbatim (same string identity is not required, but content must
	// match byte-for-byte). Confirms the ContainsAny bail-out works.
	in := "/usr/local/bin/jin"
	if got := tomlEscapeForShell(in); got != in {
		t.Errorf("tomlEscapeForShell(%q) = %q, want %q", in, got, in)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
