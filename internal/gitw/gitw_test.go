package gitw_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/arbourd/git-sync/internal/gittest"
	"github.com/arbourd/git-sync/internal/gitw"
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
	t.Chdir(t.TempDir())

	if gitw.IsGitDir() {
		t.Fatal("IsGitDir() = true, want false outside a git repo")
	}
}

func TestRemoteFromHead(t *testing.T) {
	for _, name := range []string{"origin", "my-fork", "a.b", "team/fork"} {
		t.Run(name, func(t *testing.T) {
			remote := gittest.NewBare(t)
			local := remote.Clone(t)
			if name != "origin" {
				local.Git(t, "remote", "rename", "origin", name)
				local.Git(t, "remote", "set-head", name, "main")
			}
			local.Chdir(t)

			got, err := gitw.RemoteFromHead()
			if err != nil {
				t.Fatalf("RemoteFromHead() error: %v", err)
			}
			if got != name {
				t.Fatalf("RemoteFromHead() = %q, want %q", got, name)
			}
		})
	}
}

func TestRemoteFromHead_ColorAlways(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Git(t, "config", "color.branch", "always")
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

	slices.Sort(got)
	if want := []string{"feature-a", "feature-b", "main"}; !slices.Equal(got, want) {
		t.Fatalf("LocalBranches() = %v, want branches %v", got, want)
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

func TestFastForwardIn(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	peer := remote.Clone(t)
	local.Chdir(t)

	sha := peer.Commit(t, "upstream work")
	peer.Push(t, "origin", "main")
	local.Git(t, "fetch", "origin")

	if err := gitw.FastForwardIn(local.Dir, "refs/remotes/origin/main"); err != nil {
		t.Fatalf("FastForwardIn() error: %v", err)
	}
	if got := local.Git(t, "rev-parse", "HEAD"); got != sha {
		t.Fatalf("HEAD = %q, want %q", got, sha)
	}
}

func TestFastForwardIn_Diverged(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	peer := remote.Clone(t)
	local.Chdir(t)

	peer.Commit(t, "upstream work")
	peer.Push(t, "origin", "main")
	local.Git(t, "fetch", "origin")
	sha := local.Commit(t, "local work")

	if err := gitw.FastForwardIn(local.Dir, "refs/remotes/origin/main"); err == nil {
		t.Fatal("FastForwardIn() error = nil, want error for diverged branch")
	}
	if got := local.Git(t, "rev-parse", "HEAD"); got != sha {
		t.Fatalf("HEAD = %q, want unchanged %q", got, sha)
	}
}

func TestWorktrees(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	linked := filepath.Join(t.TempDir(), "linked")
	local.Git(t, "worktree", "add", "-b", "work", linked, "main")

	got, err := gitw.Worktrees()
	if err != nil {
		t.Fatalf("Worktrees() error: %v", err)
	}
	for branch, want := range map[string]string{"main": local.Dir, "work": linked} {
		if len(got[branch]) != 1 || !samePath(got[branch][0], want) {
			t.Errorf("Worktrees()[%q] = %q, want [%q]", branch, got[branch], want)
		}
	}
	if len(got) != 2 {
		t.Errorf("Worktrees() = %v, want only main and work", got)
	}
}

func TestWorktrees_SeveralHoldingOneBranch(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	other := filepath.Join(t.TempDir(), "other")
	local.Git(t, "worktree", "add", "--force", other, "main")

	got, err := gitw.Worktrees()
	if err != nil {
		t.Fatalf("Worktrees() error: %v", err)
	}
	if len(got["main"]) != 2 {
		t.Fatalf("Worktrees()[main] = %q, want both worktrees", got["main"])
	}
}

func TestWorktrees_PathWithNewline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows paths cannot contain a newline")
	}
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	linked := filepath.Join(t.TempDir(), "a\nb")
	local.Git(t, "worktree", "add", "-b", "work", linked, "main")

	got, err := gitw.Worktrees()
	if err != nil {
		t.Fatalf("Worktrees() error: %v", err)
	}
	if len(got["work"]) != 1 || !samePath(got["work"][0], linked) {
		t.Fatalf("Worktrees()[work] = %q, want [%q]", got["work"], linked)
	}
}

func TestWorktrees_MidRebase(t *testing.T) {
	for _, backend := range []string{"--merge", "--apply"} {
		t.Run(backend, func(t *testing.T) {
			remote := gittest.NewBare(t)
			local := remote.Clone(t)
			local.Chdir(t)

			// Conflicting edits stop the rebase, leaving a detached HEAD and the backend's head-name file.
			conflict := filepath.Join(local.Dir, "conflict")
			edit := func(content string) {
				if err := os.WriteFile(conflict, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
				local.Git(t, "add", "conflict")
				local.Git(t, "commit", "-m", content)
			}
			local.CreateBranch(t, "feature")
			local.Checkout(t, "feature")
			edit("feature")
			local.Checkout(t, "main")
			edit("main")
			local.Checkout(t, "feature")

			rebase := exec.CommandContext(t.Context(), "git", "rebase", backend, "main")
			rebase.Dir = local.Dir
			if err := rebase.Run(); err == nil {
				t.Fatal("rebase succeeded, want a conflict")
			}

			got, err := gitw.Worktrees()
			if err != nil {
				t.Fatalf("Worktrees() error: %v", err)
			}
			if len(got["feature"]) != 1 || !samePath(got["feature"][0], local.Dir) {
				t.Fatalf("Worktrees()[feature] = %q, want [%q]", got["feature"], local.Dir)
			}
			if gitw.IsClean(local.Dir) {
				t.Error("IsClean() = true mid-rebase, want false")
			}
		})
	}
}

func samePath(a, b string) bool {
	a, _ = filepath.EvalSymlinks(a)
	b, _ = filepath.EvalSymlinks(b)
	return a == b
}

func TestIsClean(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)

	if !gitw.IsClean(local.Dir) {
		t.Error("IsClean() = false for a fresh clone, want true")
	}

	if err := os.WriteFile(filepath.Join(local.Dir, "junk"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !gitw.IsClean(local.Dir) {
		t.Error("IsClean() = false with only an untracked file, want true")
	}

	local.Git(t, "add", "junk")
	if gitw.IsClean(local.Dir) {
		t.Error("IsClean() = true with a staged file, want false")
	}

	if gitw.IsClean(filepath.Join(local.Dir, "missing")) {
		t.Error("IsClean() = true for a missing directory, want false")
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
	// Not checked out anywhere: update-ref on a held branch would leave its index stale.
	local.CreateBranch(t, "target")

	if err := gitw.UpdateRef("refs/heads/target", "refs/heads/feature"); err != nil {
		t.Fatalf("UpdateRef() error: %v", err)
	}
	if got := local.Git(t, "rev-parse", "refs/heads/target"); got != sha {
		t.Fatalf("target = %q, want %q", got, sha)
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

func TestWorktrees_MidBisect(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)
	local.Chdir(t)

	local.CreateBranch(t, "feature")
	local.Checkout(t, "feature")
	for _, msg := range []string{"one", "two", "three"} {
		local.Commit(t, msg)
	}
	local.Git(t, "bisect", "start", "HEAD", "HEAD~3")
	if got := local.Git(t, "rev-parse", "--abbrev-ref", "HEAD"); got != "HEAD" {
		t.Fatalf("HEAD = %q, want detached by the bisect", got)
	}

	got, err := gitw.Worktrees()
	if err != nil {
		t.Fatalf("Worktrees() error: %v", err)
	}
	if len(got["feature"]) != 1 || !samePath(got["feature"][0], local.Dir) {
		t.Fatalf("Worktrees()[feature] = %q, want [%q]", got["feature"], local.Dir)
	}
	if gitw.IsClean(local.Dir) {
		t.Error("IsClean() = true mid-bisect, want false")
	}
}

func TestIsClean_OperationInProgress(t *testing.T) {
	for _, name := range []string{"rebase-merge", "rebase-apply", "BISECT_LOG", "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "sequencer"} {
		t.Run(name, func(t *testing.T) {
			remote := gittest.NewBare(t)
			local := remote.Clone(t)

			if !gitw.IsClean(local.Dir) {
				t.Fatal("IsClean() = false before the operation started, want true")
			}
			// Git keeps these three as directories; the rest are plain files.
			marker := filepath.Join(local.Dir, ".git", name)
			switch name {
			case "rebase-merge", "rebase-apply", "sequencer":
				if err := os.Mkdir(marker, 0o755); err != nil {
					t.Fatal(err)
				}
			default:
				if err := os.WriteFile(marker, []byte("x\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if gitw.IsClean(local.Dir) {
				t.Errorf("IsClean() = true with %s present, want false", name)
			}
		})
	}
}

func TestIsClean_PausedMerge(t *testing.T) {
	remote := gittest.NewBare(t)
	local := remote.Clone(t)

	local.CreateBranch(t, "feature")
	local.Checkout(t, "feature")
	local.Commit(t, "feature work")
	local.Checkout(t, "main")
	local.Commit(t, "main work")
	local.Git(t, "merge", "--no-commit", "--no-ff", "feature")

	if gitw.IsClean(local.Dir) {
		t.Error("IsClean() = true during a paused merge, want false")
	}
}
