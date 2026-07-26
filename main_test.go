package main

import (
	"strings"
	"testing"

	"github.com/arbourd/git-sync/internal/gittest"
)

func TestSync_FastForwardNonCurrentBranch(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	peer := remote.Clone(t)

	peer.CreateBranch(t, "feature")
	peer.Checkout(t, "feature")
	peer.Commit(t, "feature start")
	peer.Push(t, "origin", "feature")

	local.Git(t, "fetch", "origin")
	local.Git(t, "checkout", "-b", "feature", "--track", "origin/feature")
	local.Checkout(t, "main")

	newSHA := peer.Commit(t, "feature more")
	peer.Push(t, "origin", "feature")

	local.Chdir(t)

	if err := sync(); err != nil {
		t.Fatalf("sync() error: %v", err)
	}

	if got := local.Git(t, "rev-parse", "refs/heads/feature"); got != newSHA {
		t.Fatalf("local refs/heads/feature = %q, want %q", got, newSHA)
	}
	if got := local.Git(t, "rev-parse", "--abbrev-ref", "HEAD"); got != "main" {
		t.Fatalf("current branch = %q, want %q (should not switch)", got, "main")
	}
}

func TestSync_FastForwardCurrentBranch(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	peer := remote.Clone(t)

	peer.CreateBranch(t, "feature")
	peer.Checkout(t, "feature")
	peer.Commit(t, "feature start")
	peer.Push(t, "origin", "feature")

	local.Git(t, "fetch", "origin")
	local.Git(t, "checkout", "-b", "feature", "--track", "origin/feature")

	newSHA := peer.Commit(t, "feature more")
	peer.Push(t, "origin", "feature")

	local.Chdir(t)

	if err := sync(); err != nil {
		t.Fatalf("sync() error: %v", err)
	}

	if got := local.Git(t, "rev-parse", "HEAD"); got != newSHA {
		t.Fatalf("local HEAD = %q, want %q", got, newSHA)
	}
	if got := local.Git(t, "rev-parse", "--abbrev-ref", "HEAD"); got != "feature" {
		t.Fatalf("current branch = %q, want %q", got, "feature")
	}
}

func TestSync_IdenticalBranchIsNoOp(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)

	wantSHA := local.Git(t, "rev-parse", "refs/heads/main")

	local.Chdir(t)

	if err := sync(); err != nil {
		t.Fatalf("sync() error: %v", err)
	}

	if got := local.Git(t, "rev-parse", "refs/heads/main"); got != wantSHA {
		t.Fatalf("local refs/heads/main = %q, want unchanged %q", got, wantSHA)
	}
	if got := local.Git(t, "rev-parse", "--abbrev-ref", "HEAD"); got != "main" {
		t.Fatalf("current branch = %q, want %q", got, "main")
	}
}

func TestSync_DeletedUpstreamMergedBranchIsDeleted(t *testing.T) {
	remote := gittest.NewBare(t)
	peer := remote.Clone(t)
	local := remote.Clone(t)

	peer.CreateBranch(t, "feature")
	peer.Push(t, "origin", "feature")

	local.Git(t, "fetch", "origin")
	local.Git(t, "checkout", "-b", "feature", "--track", "origin/feature")

	peer.Push(t, "origin", "--delete", "feature")

	local.Chdir(t)

	if err := sync(); err != nil {
		t.Fatalf("sync() error: %v", err)
	}

	if got := local.Git(t, "rev-parse", "--abbrev-ref", "HEAD"); got != "main" {
		t.Fatalf("current branch = %q, want %q (should check out default branch)", got, "main")
	}

	branches := local.Git(t, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if strings.Contains(branches, "feature") {
		t.Fatalf("local branches = %q, want %q deleted", branches, "feature")
	}
}

func TestSync_DeletedUpstreamUnmergedBranchWarnsAndKeeps(t *testing.T) {
	remote := gittest.NewBare(t)
	peer := remote.Clone(t)
	local := remote.Clone(t)

	peer.CreateBranch(t, "feature")
	peer.Checkout(t, "feature")
	featureSHA := peer.Commit(t, "feature work")
	peer.Push(t, "origin", "feature")

	local.Git(t, "fetch", "origin")
	local.Git(t, "checkout", "-b", "feature", "--track", "origin/feature")
	local.Checkout(t, "main")

	peer.Push(t, "origin", "--delete", "feature")

	local.Chdir(t)

	if err := sync(); err != nil {
		t.Fatalf("sync() error: %v", err)
	}

	if got := local.Git(t, "rev-parse", "refs/heads/feature"); got != featureSHA {
		t.Fatalf("local refs/heads/feature = %q, want unchanged %q", got, featureSHA)
	}
}

func TestSync_UntrackedBranchWithMatchingRemoteIsSynced(t *testing.T) {
	remote := gittest.NewBare(t)
	peer := remote.Clone(t)
	local := remote.Clone(t)

	peer.CreateBranch(t, "feature")
	peer.Checkout(t, "feature")
	newSHA := peer.Commit(t, "feature work")
	peer.Push(t, "origin", "feature")

	local.Git(t, "fetch", "origin")
	// Untracked: branchToRemote is unset, forcing sync() to fall back to HasFile.
	local.CreateBranch(t, "feature")

	local.Chdir(t)

	if err := sync(); err != nil {
		t.Fatalf("sync() error: %v", err)
	}

	if got := local.Git(t, "rev-parse", "refs/heads/feature"); got != newSHA {
		t.Fatalf("local refs/heads/feature = %q, want %q", got, newSHA)
	}
}

func TestSync_UntrackedBranchWithoutMatchingRemoteIsUntouched(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)

	local.CreateBranch(t, "solo")
	wantSHA := local.Git(t, "rev-parse", "refs/heads/solo")

	local.Chdir(t)

	if err := sync(); err != nil {
		t.Fatalf("sync() error: %v", err)
	}

	if got := local.Git(t, "rev-parse", "refs/heads/solo"); got != wantSHA {
		t.Fatalf("local refs/heads/solo = %q, want unchanged %q", got, wantSHA)
	}
}

func TestSync_UnpushedCommitsWarnsAndKeeps(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)

	local.CreateBranch(t, "feature")
	local.Checkout(t, "feature")
	local.Push(t, "-u", "origin", "feature")

	unpushedSHA := local.Commit(t, "local only work")

	local.Chdir(t)

	if err := sync(); err != nil {
		t.Fatalf("sync() error: %v", err)
	}

	if got := local.Git(t, "rev-parse", "refs/heads/feature"); got != unpushedSHA {
		t.Fatalf("local refs/heads/feature = %q, want unchanged %q", got, unpushedSHA)
	}
}
