package session

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateBaselineDescription exercises the pure-function layer A
// generator across all documented branches: empty input, non-git dirs, git
// roots with and without branch/subpath, and a worktree layout where .git is
// a regular file.
func TestGenerateBaselineDescription(t *testing.T) {
	// Non-git baseline: a plain temp dir with no .git anywhere upstream.
	nonGitDir := t.TempDir()

	// Isolate the git fixtures from the parent test tree (t.TempDir may sit
	// inside a real git checkout on the test host). Placing them under a
	// nested TempDir does not help because walk-up would still find the host
	// repo, so we build fixtures that override the closest .git along the way.
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	repoSubdir := filepath.Join(repoRoot, "internal", "session")
	if err := os.MkdirAll(repoSubdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	// Worktree fixture with an unresolvable gitdir: the generator should
	// gracefully fall back to the worktree directory basename rather than
	// producing a broken "<empty>:<worktree>" label.
	orphanWorktreeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(orphanWorktreeDir, ".git"), []byte("gitdir: /nowhere\n"), 0o644); err != nil {
		t.Fatalf("write orphan .git file: %v", err)
	}

	// Realistic worktree layout: a fake main repo with a worktree directory
	// whose .git file points back into <main-repo>/.git/worktrees/<name>. The
	// generator should prepend the *main repo* basename to the *worktree*
	// basename so multiple worktrees of the same repo remain distinguishable.
	mainRepoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(mainRepoDir, ".git", "worktrees", "jin-abcd1234"), 0o755); err != nil {
		t.Fatalf("mkdir main repo .git/worktrees: %v", err)
	}
	worktreeDir := filepath.Join(t.TempDir(), "jin-abcd1234")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatalf("mkdir worktree dir: %v", err)
	}
	gitFileContent := "gitdir: " + filepath.Join(mainRepoDir, ".git", "worktrees", "jin-abcd1234") + "\n"
	if err := os.WriteFile(filepath.Join(worktreeDir, ".git"), []byte(gitFileContent), 0o644); err != nil {
		t.Fatalf("write worktree .git file: %v", err)
	}

	tests := []struct {
		name          string
		workDir       string
		currentBranch string
		isWorktree    bool
		tmuxHint      string
		want          string
	}{
		{
			name: "empty workDir falls back to session",
			want: "session",
		},
		{
			name:    "non-git directory uses basename",
			workDir: nonGitDir,
			want:    filepath.Base(nonGitDir),
		},
		{
			name:    "git repo root without branch",
			workDir: repoRoot,
			want:    filepath.Base(repoRoot),
		},
		{
			name:          "git repo root with branch",
			workDir:       repoRoot,
			currentBranch: "main",
			want:          filepath.Base(repoRoot) + ":main",
		},
		{
			name:          "git repo subdir with branch and subpath",
			workDir:       repoSubdir,
			currentBranch: "feat/x",
			want:          filepath.Base(repoRoot) + ":feat/x:internal/session",
		},
		{
			name:    "git repo subdir without branch preserves subpath",
			workDir: repoSubdir,
			want:    filepath.Base(repoRoot) + ":internal/session",
		},
		{
			name:       "worktree with unresolvable gitdir falls back to worktree basename",
			workDir:    orphanWorktreeDir,
			isWorktree: true,
			want:       filepath.Base(orphanWorktreeDir),
		},
		{
			name:       "worktree prepends main repo basename",
			workDir:    worktreeDir,
			isWorktree: true,
			want:       filepath.Base(mainRepoDir) + ":jin-abcd1234",
		},
		{
			name:          "worktree with branch appends branch after worktree name",
			workDir:       worktreeDir,
			currentBranch: "wip/refactor",
			isWorktree:    true,
			want:          filepath.Base(mainRepoDir) + ":jin-abcd1234:wip/refactor",
		},
		{
			name:     "tmuxHint is ignored in this phase",
			workDir:  nonGitDir,
			tmuxHint: "should-not-appear",
			want:     filepath.Base(nonGitDir),
		},
		{
			name:    "trailing slash is cleaned before basename",
			workDir: nonGitDir + "/",
			want:    filepath.Base(nonGitDir),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GenerateBaselineDescription(tc.workDir, tc.currentBranch, tc.isWorktree, tc.tmuxHint)
			if got != tc.want {
				t.Errorf("GenerateBaselineDescription(%q, %q, %v, %q) = %q, want %q",
					tc.workDir, tc.currentBranch, tc.isWorktree, tc.tmuxHint, got, tc.want)
			}
			if got == "" {
				t.Errorf("invariant violated: returned empty string")
			}
		})
	}
}

// TestFindRepoRoot verifies the walk-up terminates correctly at both a
// discovered .git and at the filesystem root.
func TestFindRepoRoot(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	nested := filepath.Join(repoRoot, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	got, ok := findRepoRoot(nested)
	if !ok {
		t.Fatalf("findRepoRoot(%q) = _, false; want true", nested)
	}
	if filepath.Clean(got) != filepath.Clean(repoRoot) {
		t.Errorf("findRepoRoot(%q) = %q, want %q", nested, got, repoRoot)
	}

	if _, ok := findRepoRoot(""); ok {
		t.Error("findRepoRoot(\"\") should return ok=false")
	}
}
