package gitw_test

import (
	"testing"

	"github.com/arbourd/git-sync/internal/gitw"
	"github.com/arbourd/git-sync/internal/gittest"
)

func TestIsGitDir(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	if !gitw.IsGitDir() {
		t.Fatal("IsGitDir() = false, want true inside a git repo")
	}
}

func TestIsGitDir_NotGitDir(t *testing.T) {
	gittest.Chdir(t, t.TempDir())

	if gitw.IsGitDir() {
		t.Fatal("IsGitDir() = true, want false outside a git repo")
	}
}

func TestRemoteFromHead(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	got, err := gitw.RemoteFromHead()
	if err != nil {
		t.Fatalf("RemoteFromHead() error: %v", err)
	}
	if got != "origin" {
		t.Fatalf("RemoteFromHead() = %q, want %q", got, "origin")
	}
}

func TestDefaultBranch(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	got, err := gitw.DefaultBranch("origin")
	if err != nil {
		t.Fatalf("DefaultBranch() error: %v", err)
	}
	if got != "main" {
		t.Fatalf("DefaultBranch() = %q, want %q", got, "main")
	}
}

func TestCurrentBranch(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	local.CreateBranch(t, "feature")
	local.Checkout(t, "feature")

	if got := gitw.CurrentBranch(); got != "feature" {
		t.Fatalf("CurrentBranch() = %q, want %q", got, "feature")
	}
}

func TestLocalBranches(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	local.CreateBranch(t, "feature-a")
	local.CreateBranch(t, "feature-b")

	got, err := gitw.LocalBranches()
	if err != nil {
		t.Fatalf("LocalBranches() error: %v", err)
	}

	want := map[string]bool{"main": true, "feature-a": true, "feature-b": true}
	if len(got) != len(want) {
		t.Fatalf("LocalBranches() = %v, want branches %v", got, want)
	}
	for _, b := range got {
		if !want[b] {
			t.Fatalf("LocalBranches() contained unexpected branch %q", b)
		}
	}
}

func TestBranchesWithRemotes(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	got, err := gitw.BranchesWithRemotes()
	if err != nil {
		t.Fatalf("BranchesWithRemotes() error: %v", err)
	}
	if got["main"] != "origin" {
		t.Fatalf(`BranchesWithRemotes()["main"] = %q, want "origin"`, got["main"])
	}
}

func TestHasFile(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	if !gitw.HasFile("refs", "remotes", "origin", "main") {
		t.Fatal(`HasFile("refs","remotes","origin","main") = false, want true`)
	}
	if gitw.HasFile("refs", "remotes", "origin", "does-not-exist") {
		t.Fatal(`HasFile("refs","remotes","origin","does-not-exist") = true, want false`)
	}
}

func TestNewRange(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	local.CreateBranch(t, "feature")
	local.Checkout(t, "feature")
	sha := local.Commit(t, "feature work")

	rng, err := gitw.NewRange("refs/heads/main", "refs/heads/feature")
	if err != nil {
		t.Fatalf("NewRange() error: %v", err)
	}
	if rng.B != sha {
		t.Fatalf("NewRange().B = %q, want %q", rng.B, sha)
	}
	if rng.IsIdentical() {
		t.Fatal("IsIdentical() = true, want false")
	}
	if !rng.IsAncestor() {
		t.Fatal("IsAncestor() = false, want true (main is an ancestor of feature)")
	}
}

func TestNewRange_Identical(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	rng, err := gitw.NewRange("refs/heads/main", "refs/remotes/origin/main")
	if err != nil {
		t.Fatalf("NewRange() error: %v", err)
	}
	if !rng.IsIdentical() {
		t.Fatal("IsIdentical() = false, want true")
	}
}
