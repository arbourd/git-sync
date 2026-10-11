package main

import (
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/arbourd/git-sync/internal/gittest"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// runSync calls sync() from the current directory and returns its error along
// with everything it printed to stdout, ANSI colors stripped.
func runSync(t *testing.T) (string, error) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	syncErr := sync()
	w.Close()
	return ansi.ReplaceAllString(<-done, ""), syncErr
}

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
	oldSHA := local.Git(t, "rev-parse", "refs/heads/feature")

	newSHA := peer.Commit(t, "feature more")
	peer.Push(t, "origin", "feature")

	local.Chdir(t)

	out, err := runSync(t)
	if err != nil {
		t.Fatalf("sync() error: %v\noutput: %s", err, out)
	}
	if want := "Updated branch feature (was " + oldSHA[:7] + ")"; !strings.Contains(out, want) {
		t.Fatalf("output = %q, want it to contain %q", out, want)
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
	oldSHA := local.Git(t, "rev-parse", "HEAD")

	newSHA := peer.Commit(t, "feature more")
	peer.Push(t, "origin", "feature")

	local.Chdir(t)

	out, err := runSync(t)
	if err != nil {
		t.Fatalf("sync() error: %v\noutput: %s", err, out)
	}
	if want := "Updated branch feature (was " + oldSHA[:7] + ")"; !strings.Contains(out, want) {
		t.Fatalf("output = %q, want it to contain %q", out, want)
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

	out, err := runSync(t)
	if err != nil {
		t.Fatalf("sync() error: %v\noutput: %s", err, out)
	}
	if out != "" {
		t.Fatalf("output = %q, want none", out)
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

	out, err := runSync(t)
	if err != nil {
		t.Fatalf("sync() error: %v\noutput: %s", err, out)
	}
	if want := "Deleted branch feature (was "; !strings.Contains(out, want) {
		t.Fatalf("output = %q, want it to contain %q", out, want)
	}

	if got := local.Git(t, "rev-parse", "--abbrev-ref", "HEAD"); got != "main" {
		t.Fatalf("current branch = %q, want %q (should check out default branch)", got, "main")
	}
	if got := local.Git(t, "for-each-ref", "refs/remotes/origin/feature"); got != "" {
		t.Fatalf("refs/remotes/origin/feature = %q, want pruned", got)
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

	out, err := runSync(t)
	if err != nil {
		t.Fatalf("sync() error: %v\noutput: %s", err, out)
	}
	if want := "warning: 'feature' was deleted on origin, but appears not merged into 'main'"; !strings.Contains(out, want) {
		t.Fatalf("output = %q, want it to contain %q", out, want)
	}

	if got := local.Git(t, "rev-parse", "refs/heads/feature"); got != featureSHA {
		t.Fatalf("local refs/heads/feature = %q, want unchanged %q", got, featureSHA)
	}
	if got := local.Git(t, "for-each-ref", "refs/remotes/origin/feature"); got != "" {
		t.Fatalf("refs/remotes/origin/feature = %q, want pruned", got)
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

	out, err := runSync(t)
	if err != nil {
		t.Fatalf("sync() error: %v\noutput: %s", err, out)
	}
	if want := "Updated branch feature"; !strings.Contains(out, want) {
		t.Fatalf("output = %q, want it to contain %q", out, want)
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

	out, err := runSync(t)
	if err != nil {
		t.Fatalf("sync() error: %v\noutput: %s", err, out)
	}
	if out != "" {
		t.Fatalf("output = %q, want none", out)
	}

	if got := local.Git(t, "rev-parse", "refs/heads/solo"); got != wantSHA {
		t.Fatalf("local refs/heads/solo = %q, want unchanged %q", got, wantSHA)
	}
}

func TestSync_ForceMovedTagIsIgnored(t *testing.T) {
	remote := gittest.NewBare(t)
	peer := remote.Clone(t)
	local := remote.Clone(t)

	// Reproduces the failure: fetch.pruneTags hard-fails on a moved tag.
	local.Git(t, "config", "fetch.pruneTags", "true")

	peer.Git(t, "tag", "nightly")
	peer.Push(t, "origin", "nightly")

	// Cache the tag locally at its original commit.
	local.Git(t, "fetch", "origin", "tag", "nightly")

	// Force-move the tag upstream, as a rolling release tag would.
	newSHA := peer.Commit(t, "new nightly commit")
	peer.Push(t, "origin", "main")
	peer.Git(t, "tag", "-f", "nightly")
	peer.Push(t, "origin", "nightly", "--force")

	local.Chdir(t)

	if out, err := runSync(t); err != nil {
		t.Fatalf("sync() error: %v, want nil (force-moved tags should not fail sync)\noutput: %s", err, out)
	}

	if got := local.Git(t, "rev-parse", "refs/remotes/origin/main"); got != newSHA {
		t.Fatalf("local refs/remotes/origin/main = %q, want %q", got, newSHA)
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

	out, err := runSync(t)
	if err != nil {
		t.Fatalf("sync() error: %v\noutput: %s", err, out)
	}
	if want := "warning: 'feature' seems to contain unpushed commits"; !strings.Contains(out, want) {
		t.Fatalf("output = %q, want it to contain %q", out, want)
	}

	if got := local.Git(t, "rev-parse", "refs/heads/feature"); got != unpushedSHA {
		t.Fatalf("local refs/heads/feature = %q, want unchanged %q", got, unpushedSHA)
	}
}

func TestSync_DivergedBranchWarnsAndKeeps(t *testing.T) {
	remote := gittest.NewBare(t)
	peer := remote.Clone(t)
	local := remote.Clone(t)

	peer.CreateBranch(t, "feature")
	peer.Push(t, "origin", "feature")

	local.Git(t, "fetch", "origin")
	local.Git(t, "checkout", "-b", "feature", "--track", "origin/feature")
	localSHA := local.Commit(t, "local work")

	peer.Checkout(t, "feature")
	peer.Commit(t, "remote work")
	peer.Push(t, "origin", "feature")

	local.Chdir(t)

	out, err := runSync(t)
	if err != nil {
		t.Fatalf("sync() error: %v\noutput: %s", err, out)
	}
	if want := "warning: 'feature' seems to contain unpushed commits"; !strings.Contains(out, want) {
		t.Fatalf("output = %q, want it to contain %q", out, want)
	}

	if got := local.Git(t, "rev-parse", "refs/heads/feature"); got != localSHA {
		t.Fatalf("local refs/heads/feature = %q, want unchanged %q", got, localSHA)
	}
	if got := local.Git(t, "rev-parse", "--abbrev-ref", "HEAD"); got != "feature" {
		t.Fatalf("current branch = %q, want %q", got, "feature")
	}
}

func TestSync_DeletedUpstreamUnmergedCurrentBranchStays(t *testing.T) {
	remote := gittest.NewBare(t)
	peer := remote.Clone(t)
	local := remote.Clone(t)

	local.CreateBranch(t, "feature")
	local.Checkout(t, "feature")
	featureSHA := local.Commit(t, "feature work")
	local.Push(t, "-u", "origin", "feature")

	peer.Git(t, "fetch", "origin")
	peer.Push(t, "origin", "--delete", "feature")

	local.Chdir(t)

	out, err := runSync(t)
	if err != nil {
		t.Fatalf("sync() error: %v\noutput: %s", err, out)
	}
	if want := "warning: 'feature' was deleted on origin, but appears not merged into 'main'"; !strings.Contains(out, want) {
		t.Fatalf("output = %q, want it to contain %q", out, want)
	}

	if got := local.Git(t, "rev-parse", "--abbrev-ref", "HEAD"); got != "feature" {
		t.Fatalf("current branch = %q, want %q (should not switch away from unmerged work)", got, "feature")
	}
	if got := local.Git(t, "rev-parse", "refs/heads/feature"); got != featureSHA {
		t.Fatalf("local refs/heads/feature = %q, want unchanged %q", got, featureSHA)
	}
}

func TestSync_MultipleBranchesAreHandledIndependently(t *testing.T) {
	remote := gittest.NewBare(t)
	peer := remote.Clone(t)
	local := remote.Clone(t)

	peer.CreateBranch(t, "behind")
	peer.CreateBranch(t, "merged")
	peer.CreateBranch(t, "ahead")
	peer.Push(t, "origin", "behind")
	peer.Push(t, "origin", "merged")
	peer.Push(t, "origin", "ahead")

	local.Git(t, "fetch", "origin")
	local.Git(t, "branch", "--track", "behind", "origin/behind")
	local.Git(t, "branch", "--track", "merged", "origin/merged")
	local.Git(t, "branch", "--track", "ahead", "origin/ahead")
	local.CreateBranch(t, "solo")
	soloSHA := local.Git(t, "rev-parse", "refs/heads/solo")

	local.Checkout(t, "ahead")
	aheadSHA := local.Commit(t, "unpushed")
	local.Checkout(t, "main")

	peer.Checkout(t, "behind")
	behindSHA := peer.Commit(t, "behind more")
	peer.Push(t, "origin", "behind")
	peer.Push(t, "origin", "--delete", "merged")

	local.Chdir(t)

	out, err := runSync(t)
	if err != nil {
		t.Fatalf("sync() error: %v\noutput: %s", err, out)
	}
	if want := "Updated branch behind"; !strings.Contains(out, want) {
		t.Fatalf("output = %q, want it to contain %q", out, want)
	}
	if want := "Deleted branch merged"; !strings.Contains(out, want) {
		t.Fatalf("output = %q, want it to contain %q", out, want)
	}
	if want := "warning: 'ahead' seems to contain unpushed commits"; !strings.Contains(out, want) {
		t.Fatalf("output = %q, want it to contain %q", out, want)
	}

	if got := local.Git(t, "rev-parse", "refs/heads/behind"); got != behindSHA {
		t.Fatalf("local refs/heads/behind = %q, want %q", got, behindSHA)
	}
	if got := local.Git(t, "for-each-ref", "refs/heads/merged"); got != "" {
		t.Fatalf("local refs/heads/merged = %q, want deleted", got)
	}
	if got := local.Git(t, "rev-parse", "refs/heads/ahead"); got != aheadSHA {
		t.Fatalf("local refs/heads/ahead = %q, want unchanged %q", got, aheadSHA)
	}
	if got := local.Git(t, "rev-parse", "refs/heads/solo"); got != soloSHA {
		t.Fatalf("local refs/heads/solo = %q, want unchanged %q", got, soloSHA)
	}
}

func TestSync_NotAGitRepository(t *testing.T) {
	gittest.NewBare(t) // isolates the git environment
	gittest.Chdir(t, t.TempDir())

	out, err := runSync(t)
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("sync() error = %v, want %q\noutput: %s", err, "not a git repository", out)
	}
}

func TestSync_NoRemoteFails(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Git(t, "remote", "remove", "origin")

	wantSHA := local.Git(t, "rev-parse", "refs/heads/main")
	local.Chdir(t)

	if out, err := runSync(t); err == nil {
		t.Fatalf("sync() error = nil, want error for repo without a remote\noutput: %s", out)
	}
	if got := local.Git(t, "rev-parse", "refs/heads/main"); got != wantSHA {
		t.Fatalf("local refs/heads/main = %q, want unchanged %q", got, wantSHA)
	}
}

func TestSync_FetchFailureReturnsErrorAndKeepsBranches(t *testing.T) {
	remote := gittest.NewBare(t)
	peer := remote.Clone(t)
	local := remote.Clone(t)

	peer.Commit(t, "main more")
	peer.Push(t, "origin", "main")

	wantSHA := local.Git(t, "rev-parse", "refs/heads/main")
	local.Git(t, "remote", "set-url", "origin", local.Dir+"/does-not-exist")
	local.Chdir(t)

	if out, err := runSync(t); err == nil {
		t.Fatalf("sync() error = nil, want fetch error\noutput: %s", out)
	}
	if got := local.Git(t, "rev-parse", "refs/heads/main"); got != wantSHA {
		t.Fatalf("local refs/heads/main = %q, want unchanged %q", got, wantSHA)
	}
}
