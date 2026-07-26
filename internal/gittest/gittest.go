// Package gittest builds disposable, isolated git repositories for tests.
package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Repo is a real, disposable git repository used for tests.
type Repo struct {
	Dir string
}

// NewBare creates a bare repository with a single initial commit on "main".
func NewBare(t testing.TB) *Repo {
	t.Helper()
	isolate(t)

	dir := t.TempDir()
	runIn(t, dir, "init", "--bare", "-b", "main")

	// Bare repos can't be committed into directly, so seed via a throwaway clone.
	seed := t.TempDir()
	runIn(t, seed, "init", "-b", "main")
	runIn(t, seed, "commit", "--allow-empty", "-m", "initial commit")
	runIn(t, seed, "push", dir, "main")

	return &Repo{Dir: dir}
}

// Clone clones r into a fresh temp directory, tracked as "origin".
func (r *Repo) Clone(t testing.TB) *Repo {
	t.Helper()
	isolate(t)

	dir := t.TempDir()
	runIn(t, dir, "clone", r.Dir, ".")

	return &Repo{Dir: dir}
}

// Commit creates an empty commit with msg and returns its full SHA.
func (r *Repo) Commit(t testing.TB, msg string) string {
	t.Helper()
	runIn(t, r.Dir, "commit", "--allow-empty", "-m", msg)
	return strings.TrimSpace(runIn(t, r.Dir, "rev-parse", "HEAD"))
}

// CreateBranch creates a new branch at HEAD without switching to it.
func (r *Repo) CreateBranch(t testing.TB, name string) {
	t.Helper()
	runIn(t, r.Dir, "branch", name)
}

// Checkout switches the working tree to the given branch.
func (r *Repo) Checkout(t testing.TB, name string) {
	t.Helper()
	runIn(t, r.Dir, "checkout", "--quiet", name)
}

// Push runs `git push` with the given args in r.
func (r *Repo) Push(t testing.TB, args ...string) {
	t.Helper()
	runIn(t, r.Dir, append([]string{"push"}, args...)...)
}

// Git runs an arbitrary git command in r and returns trimmed stdout.
func (r *Repo) Git(t testing.TB, args ...string) string {
	t.Helper()
	return strings.TrimSpace(runIn(t, r.Dir, args...))
}

// Chdir makes dir the process's cwd for the rest of the test.
func Chdir(t testing.TB, dir string) {
	t.Helper()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("gittest: os.Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("gittest: os.Chdir(%s): %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("gittest: restore os.Chdir(%s): %v", orig, err)
		}
	})
}

// Chdir makes r.Dir the process's cwd for the rest of the test.
func (r *Repo) Chdir(t testing.TB) {
	t.Helper()
	Chdir(t, r.Dir)
}

// isolate pins git's environment to per-test temp locations.
func isolate(t testing.TB) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "git-sync tests")
	t.Setenv("GIT_AUTHOR_EMAIL", "git-sync-tests@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "git-sync tests")
	t.Setenv("GIT_COMMITTER_EMAIL", "git-sync-tests@example.com")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
}

func runIn(t testing.TB, dir string, args ...string) string {
	t.Helper()

	var stdout, stderr strings.Builder
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		output := stdout.String() + stderr.String()
		t.Fatalf("gittest: git %s (in %s): %v\n%s", strings.Join(args, " "), dir, err, output)
	}
	return stdout.String()
}
