package gittest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arbourd/git-sync/internal/gittest"
)

func TestNewBareAndClone(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)

	if got := local.Git(t, "rev-parse", "--abbrev-ref", "HEAD"); got != "main" {
		t.Fatalf("current branch = %q, want %q", got, "main")
	}
	if got := local.Git(t, "remote"); got != "origin" {
		t.Fatalf("remote = %q, want %q", got, "origin")
	}
	if got := local.Git(t, "symbolic-ref", "refs/remotes/origin/HEAD"); got != "refs/remotes/origin/main" {
		t.Fatalf("origin/HEAD = %q, want %q", got, "refs/remotes/origin/main")
	}
}

func TestCommitCreateBranchCheckout(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)

	local.CreateBranch(t, "feature")
	local.Checkout(t, "feature")
	sha := local.Commit(t, "feature work")

	if got := local.Git(t, "rev-parse", "HEAD"); got != sha {
		t.Fatalf("HEAD = %q, want %q", got, sha)
	}
	if got := local.Git(t, "rev-parse", "--abbrev-ref", "HEAD"); got != "feature" {
		t.Fatalf("current branch = %q, want %q", got, "feature")
	}
}

func TestPush(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)

	local.CreateBranch(t, "feature")
	local.Checkout(t, "feature")
	sha := local.Commit(t, "feature work")
	local.Push(t, "origin", "feature")

	if got := remote.Git(t, "rev-parse", "refs/heads/feature"); got != sha {
		t.Fatalf("remote refs/heads/feature = %q, want %q", got, sha)
	}
}

func TestChdir(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}

	local.Chdir(t)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}

	wantDir, err := filepath.EvalSymlinks(local.Dir)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%s): %v", local.Dir, err)
	}
	gotDir, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%s): %v", cwd, err)
	}
	if gotDir != wantDir {
		t.Fatalf("cwd = %q, want %q", gotDir, wantDir)
	}
	if orig == cwd {
		t.Fatalf("cwd did not change from %q", orig)
	}
}
