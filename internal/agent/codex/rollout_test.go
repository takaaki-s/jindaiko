package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	fixtureBasic    = "testdata/rollout-basic.jsonl"
	fixtureMetaOnly = "testdata/rollout-meta-only.jsonl"
	fixtureBroken   = "testdata/rollout-broken.jsonl"
	fixtureNoUser   = "testdata/rollout-no-user.jsonl"
	fixtureEmpty    = "testdata/rollout-empty.jsonl"

	basicUUID = "01900000-0000-7000-8000-000000000abc"
)

// stageRollout copies src (a testdata fixture) into a fake Codex sessions dir
// under root, at the given shard (yyyy/mm/dd) and with the given UUID embedded
// in the filename. Returns the absolute path of the staged file.
func stageRollout(t *testing.T, root, shard, uuid, src string) string {
	t.Helper()
	dir := filepath.Join(root, shard)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture %s: %v", src, err)
	}
	dst := filepath.Join(dir, "rollout-2026-07-11T06-25-10-"+uuid+".jsonl")
	if err := os.WriteFile(dst, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
	return dst
}

func TestLocator_HomeFallback(t *testing.T) {
	// CODEX_HOME unset → SessionsDir is <home>/.codex/sessions
	t.Setenv("CODEX_HOME", "")
	loc := NewLocator("/opt/home")
	want := filepath.Join("/opt/home", ".codex", "sessions")
	if loc.SessionsDir != want {
		t.Errorf("SessionsDir = %q, want %q", loc.SessionsDir, want)
	}
}

func TestLocator_CodexHomeOverride(t *testing.T) {
	t.Setenv("CODEX_HOME", "/xdg/codex")
	loc := NewLocator("/opt/home")
	want := filepath.Join("/xdg/codex", "sessions")
	if loc.SessionsDir != want {
		t.Errorf("SessionsDir = %q, want %q", loc.SessionsDir, want)
	}
}

func TestLocator_Find_Hit(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	staged := stageRollout(t, filepath.Join(root, "sessions"), "2026/07/11", basicUUID, fixtureBasic)

	got, ok := NewLocator("").Find(basicUUID)
	if !ok {
		t.Fatalf("Find(%q) = _, false, want ok=true", basicUUID)
	}
	if got != staged {
		t.Errorf("Find = %q, want %q", got, staged)
	}
}

func TestLocator_Find_Miss(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	// Stage one file with a different UUID so the sessions dir exists.
	stageRollout(t, filepath.Join(root, "sessions"), "2026/07/11", "01900000-0000-7000-8000-decoydecoy00", fixtureBasic)

	if got, ok := NewLocator("").Find(basicUUID); ok {
		t.Errorf("Find(missing UUID) = (%q, true), want false", got)
	}
}

func TestLocator_Find_EmptyUUID(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	stageRollout(t, filepath.Join(root, "sessions"), "2026/07/11", basicUUID, fixtureBasic)

	if got, ok := NewLocator("").Find(""); ok {
		t.Errorf("Find(empty) = (%q, true), want false", got)
	}
}

func TestLocator_Find_MultipleHits_NewestWins(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	sessions := filepath.Join(root, "sessions")

	older := stageRollout(t, sessions, "2026/07/10", basicUUID, fixtureBasic)
	newer := stageRollout(t, sessions, "2026/07/11", basicUUID, fixtureBasic)

	// Force mtimes: older is 2h behind, newer is now-ish.
	now := time.Now()
	if err := os.Chtimes(older, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("chtimes older: %v", err)
	}
	if err := os.Chtimes(newer, now, now); err != nil {
		t.Fatalf("chtimes newer: %v", err)
	}

	got, ok := NewLocator("").Find(basicUUID)
	if !ok {
		t.Fatalf("Find = _, false, want ok=true")
	}
	if got != newer {
		t.Errorf("Find = %q, want %q (newest by mtime)", got, newer)
	}
}

func TestLocator_Find_NilReceiverAndEmptyDir(t *testing.T) {
	// Nil-safety guard: a caller who dropped a Locator should not panic.
	var loc *Locator
	if _, ok := loc.Find(basicUUID); ok {
		t.Errorf("nil Locator Find returned ok=true")
	}
	loc = &Locator{SessionsDir: ""}
	if _, ok := loc.Find(basicUUID); ok {
		t.Errorf("empty SessionsDir Find returned ok=true")
	}
}

func TestReadMeta_OK(t *testing.T) {
	meta, err := ReadMeta(fixtureBasic)
	if err != nil {
		t.Fatalf("ReadMeta: %v", err)
	}
	if meta.ID != basicUUID {
		t.Errorf("ID = %q, want %q", meta.ID, basicUUID)
	}
	if meta.Cwd != "/tmp/example" {
		t.Errorf("Cwd = %q, want %q", meta.Cwd, "/tmp/example")
	}
}

func TestReadMeta_MetaOnlyFile(t *testing.T) {
	meta, err := ReadMeta(fixtureMetaOnly)
	if err != nil {
		t.Fatalf("ReadMeta: %v", err)
	}
	if meta.ID != "01900000-0000-7000-8000-000000000def" {
		t.Errorf("ID = %q", meta.ID)
	}
	if meta.Cwd != "/tmp/meta-only" {
		t.Errorf("Cwd = %q", meta.Cwd)
	}
}

func TestReadMeta_EmptyFile(t *testing.T) {
	if _, err := ReadMeta(fixtureEmpty); err == nil {
		t.Error("ReadMeta(empty) = nil error, want non-nil")
	}
}

func TestReadMeta_NonexistentFile(t *testing.T) {
	if _, err := ReadMeta(filepath.Join(t.TempDir(), "nonexistent.jsonl")); err == nil {
		t.Error("ReadMeta(missing) = nil error, want non-nil")
	}
}

func TestReadMeta_FirstLineNotMeta(t *testing.T) {
	// Write a file whose first line is a valid but non-session_meta row.
	path := filepath.Join(t.TempDir(), "wrong-first-line.jsonl")
	body := `{"type":"event_msg","payload":{"type":"task_started"}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := ReadMeta(path); err == nil {
		t.Error("ReadMeta(wrong first line) = nil error, want non-nil")
	}
}

func TestFirstUserPrompt_OK(t *testing.T) {
	got, ok := FirstUserPrompt(fixtureBasic)
	if !ok {
		t.Fatalf("FirstUserPrompt = _, false, want ok=true")
	}
	if got != "Hello, echo the current date" {
		t.Errorf("got = %q, want %q", got, "Hello, echo the current date")
	}
}

func TestFirstUserPrompt_SkipsEnvironmentContext(t *testing.T) {
	// The basic fixture has the pseudo-user env row BEFORE the real prompt,
	// so a passing TestFirstUserPrompt_OK already covers this — but call
	// FirstUserPromptFrom out on a stripped-down inline fixture too, so a
	// future edit to the shared fixture cannot regress the skip logic
	// without failing this test.
	body := strings.Join([]string{
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>skip me</environment_context>"}]}}`,
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"real prompt"}]}}`,
	}, "\n")
	got, ok := firstUserPromptFrom(strings.NewReader(body))
	if !ok || got != "real prompt" {
		t.Errorf("got = (%q, %v), want (%q, true)", got, ok, "real prompt")
	}
}

func TestFirstUserPrompt_SkipsDeveloperAndPseudoUser(t *testing.T) {
	got, ok := FirstUserPrompt(fixtureNoUser)
	if ok {
		t.Errorf("FirstUserPrompt = (%q, true), want (\"\", false) — file has only dev + env_context rows", got)
	}
}

func TestFirstUserPrompt_TolratesBrokenLines(t *testing.T) {
	got, ok := FirstUserPrompt(fixtureBroken)
	if !ok {
		t.Fatalf("FirstUserPrompt = _, false, want ok=true (broken lines must be skipped)")
	}
	if got != "prompt after broken lines" {
		t.Errorf("got = %q, want %q", got, "prompt after broken lines")
	}
}

func TestFirstUserPrompt_EmptyFile(t *testing.T) {
	if got, ok := FirstUserPrompt(fixtureEmpty); ok {
		t.Errorf("FirstUserPrompt(empty) = (%q, true), want false", got)
	}
}

func TestFirstUserPrompt_NonexistentFile(t *testing.T) {
	if got, ok := FirstUserPrompt(filepath.Join(t.TempDir(), "nope.jsonl")); ok {
		t.Errorf("FirstUserPrompt(missing) = (%q, true), want false", got)
	}
}

func TestFirstUserPrompt_LongLine(t *testing.T) {
	// A 128 KiB prompt (well above bufio.Scanner's default 64 KiB buffer)
	// exercises the Buffer expansion. The parser must return it in full.
	huge := strings.Repeat("x", 128*1024)
	body := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"` + huge + `"}]}}` + "\n"
	got, ok := firstUserPromptFrom(strings.NewReader(body))
	if !ok {
		t.Fatalf("FirstUserPrompt(huge) = _, false, want ok=true")
	}
	if len(got) != len(huge) {
		t.Errorf("len(got) = %d, want %d", len(got), len(huge))
	}
}

func TestFirstUserPrompt_ReturnsFirstOnlyNotSecond(t *testing.T) {
	// The basic fixture intentionally contains a SECOND user prompt after
	// the first — Layer C-transcript wants the first turn, so the parser
	// must return early rather than iterate to the last.
	got, ok := FirstUserPrompt(fixtureBasic)
	if !ok {
		t.Fatalf("FirstUserPrompt = _, false")
	}
	if got == "Second user prompt, must NOT be returned by FirstUserPrompt" {
		t.Error("FirstUserPrompt returned the second prompt; must return the first")
	}
}
