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

func TestFetch(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	peer := remote.Clone(t)
	local.Chdir(t)

	sha := peer.Commit(t, "upstream work")
	peer.Push(t, "origin", "main")

	if err := gitw.Fetch("origin"); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if got := local.Git(t, "rev-parse", "refs/remotes/origin/main"); got != sha {
		t.Fatalf("origin/main = %q, want %q", got, sha)
	}
}

func TestFetch_PrunesDeletedBranches(t *testing.T) {
	remote := gittest.NewBare(t)
	peer := remote.Clone(t)
	peer.CreateBranch(t, "feature")
	peer.Push(t, "origin", "feature")
	local := remote.Clone(t)
	local.Chdir(t)

	peer.Push(t, "origin", "--delete", "feature")

	if err := gitw.Fetch("origin"); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if got := local.Git(t, "for-each-ref", "refs/remotes/origin/feature"); got != "" {
		t.Fatalf("refs/remotes/origin/feature still exists: %q", got)
	}
}

func TestFetch_UnknownRemote(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	if err := gitw.Fetch("nope"); err == nil {
		t.Fatal("Fetch() error = nil, want error for unknown remote")
	}
}

func TestUpstream(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	got, err := gitw.Upstream("main")
	if err != nil {
		t.Fatalf("Upstream() error: %v", err)
	}
	if want := "refs/remotes/origin/main"; got != want {
		t.Fatalf("Upstream() = %q, want %q", got, want)
	}
}

func TestUpstream_NoUpstream(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	local.CreateBranch(t, "feature")

	if got, err := gitw.Upstream("feature"); err == nil {
		t.Fatalf("Upstream() = %q, want error for branch without upstream", got)
	}
}

func TestFastForwardCurrent(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	peer := remote.Clone(t)
	local.Chdir(t)

	sha := peer.Commit(t, "upstream work")
	peer.Push(t, "origin", "main")
	local.Git(t, "fetch", "origin")

	if err := gitw.FastForwardCurrent("refs/remotes/origin/main"); err != nil {
		t.Fatalf("FastForwardCurrent() error: %v", err)
	}
	if got := local.Git(t, "rev-parse", "HEAD"); got != sha {
		t.Fatalf("HEAD = %q, want %q", got, sha)
	}
}

func TestFastForwardCurrent_Diverged(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	peer := remote.Clone(t)
	local.Chdir(t)

	peer.Commit(t, "upstream work")
	peer.Push(t, "origin", "main")
	local.Git(t, "fetch", "origin")
	sha := local.Commit(t, "local work")

	if err := gitw.FastForwardCurrent("refs/remotes/origin/main"); err == nil {
		t.Fatal("FastForwardCurrent() error = nil, want error for diverged branch")
	}
	if got := local.Git(t, "rev-parse", "HEAD"); got != sha {
		t.Fatalf("HEAD = %q, want unchanged %q", got, sha)
	}
}

func TestUpdateRef(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	local.CreateBranch(t, "feature")
	local.Checkout(t, "feature")
	sha := local.Commit(t, "feature work")
	local.Checkout(t, "main")

	if err := gitw.UpdateRef("refs/heads/main", "refs/heads/feature"); err != nil {
		t.Fatalf("UpdateRef() error: %v", err)
	}
	if got := local.Git(t, "rev-parse", "refs/heads/main"); got != sha {
		t.Fatalf("main = %q, want %q", got, sha)
	}
}

func TestUpdateRef_UnknownTarget(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	if err := gitw.UpdateRef("refs/heads/main", "refs/heads/does-not-exist"); err == nil {
		t.Fatal("UpdateRef() error = nil, want error for unknown target")
	}
}

func TestCheckout(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	local.CreateBranch(t, "feature")

	if err := gitw.Checkout("feature"); err != nil {
		t.Fatalf("Checkout() error: %v", err)
	}
	if got := local.Git(t, "rev-parse", "--abbrev-ref", "HEAD"); got != "feature" {
		t.Fatalf("HEAD = %q, want %q", got, "feature")
	}
}

func TestCheckout_UnknownBranch(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	if err := gitw.Checkout("does-not-exist"); err == nil {
		t.Fatal("Checkout() error = nil, want error for unknown branch")
	}
}

func TestDeleteBranch(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	local.CreateBranch(t, "feature")

	if err := gitw.DeleteBranch("feature"); err != nil {
		t.Fatalf("DeleteBranch() error: %v", err)
	}
	if got := local.Git(t, "for-each-ref", "refs/heads/feature"); got != "" {
		t.Fatalf("refs/heads/feature still exists: %q", got)
	}
}

func TestDeleteBranch_Unmerged(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	local.CreateBranch(t, "feature")
	local.Checkout(t, "feature")
	local.Commit(t, "unmerged work")
	local.Checkout(t, "main")

	if err := gitw.DeleteBranch("feature"); err != nil {
		t.Fatalf("DeleteBranch() error: %v", err)
	}
	if got := local.Git(t, "for-each-ref", "refs/heads/feature"); got != "" {
		t.Fatalf("refs/heads/feature still exists: %q", got)
	}
}

func TestDeleteBranch_Current(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	if err := gitw.DeleteBranch("main"); err == nil {
		t.Fatal("DeleteBranch() error = nil, want error for the checked out branch")
	}
}
