package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/arbourd/git-sync/internal/gitw"
)

const (
	green      = "\033[32m"
	lightGreen = "\033[32;1m"
	red        = "\033[31m"
	lightRed   = "\033[31;1m"
	resetColor = "\033[0m"
)

// Version is set via -ldflags at build time.
var Version = "dev"

const usage = `Usage: git-sync [options]

Update all local branches from their remotes.

Options:
  -h, --help     Show this help message
  -v, --version  Show version`

func main() {
	done, err := processArgs(os.Args[1:], os.Stdout)
	if err == nil && !done {
		err = sync()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", formatError(err))
		os.Exit(1)
	}
}

// processArgs reports done if it printed help or the version.
// Help and version win wherever they appear before "--".
func processArgs(args []string, stdout io.Writer) (done bool, err error) {
	for _, a := range args {
		if a == "--" {
			break
		}
		switch a {
		case "-h", "--help":
			_, err = fmt.Fprintln(stdout, usage)
			return true, err
		case "-v", "--version":
			_, err = fmt.Fprintf(stdout, "git-sync %s\n", Version)
			return true, err
		}
	}

	// No positional arguments.
	for i, a := range args {
		switch {
		case a == "--":
			if i+1 < len(args) {
				return false, fmt.Errorf("too many arguments: %q\n\n%s", args[i+1], usage)
			}
			return false, nil
		case strings.HasPrefix(a, "-"):
			return false, fmt.Errorf("unknown option %q\n\n%s", a, usage)
		default:
			return false, fmt.Errorf("too many arguments: %q\n\n%s", a, usage)
		}
	}
	return false, nil
}

// formatError trims err and strips a leading "fatal: ".
func formatError(err error) string {
	return strings.TrimPrefix(strings.TrimSpace(err.Error()), "fatal: ")
}

// syncer holds state shared across branches.
type syncer struct {
	remote        string
	defaultBranch string
	currentBranch string
	worktrees     map[string]string // branch -> worktree path
}

func sync() error {
	if !gitw.IsGitDir() {
		return errors.New("not a git repository")
	}

	remote, err := gitw.RemoteFromHead()
	if err != nil {
		return err
	}

	defaultBranch, err := gitw.DefaultBranch(remote)
	if err != nil {
		return err
	}

	worktrees, err := gitw.Worktrees()
	if err != nil {
		return err
	}

	s := &syncer{
		remote:        remote,
		defaultBranch: defaultBranch,
		currentBranch: gitw.CurrentBranch(),
		worktrees:     worktrees,
	}

	if err := gitw.Fetch(remote); err != nil {
		return err
	}

	branches, err := gitw.LocalBranches()
	if err != nil {
		return err
	}

	tracking, err := gitw.BranchesWithRemotes()
	if err != nil {
		return err
	}

	for _, branch := range branches {
		upstream, gone := s.upstream(branch, tracking[branch] == s.remote)

		var err error
		switch {
		case upstream != "":
			err = s.update(branch, upstream)
		case gone:
			err = s.prune(branch)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// upstream returns the remote ref branch should match.
// gone is true if a tracked branch's upstream was deleted.
func (s *syncer) upstream(branch string, tracked bool) (upstream string, gone bool) {
	if tracked {
		upstream, err := gitw.Upstream(branch)
		return upstream, err != nil
	}

	candidate := fmt.Sprintf("refs/remotes/%s/%s", s.remote, branch)
	if gitw.HasFile(candidate) {
		return candidate, false
	}
	return "", false
}

// update fast-forwards branch to upstream, or warns if it has local commits.
func (s *syncer) update(branch, upstream string) error {
	ref := localRef(branch)
	diff, err := gitw.NewRange(ref, upstream)
	if err != nil {
		return err
	}

	switch {
	case diff.IsIdentical():
		return nil
	case !diff.IsAncestor():
		fmt.Printf("warning: '%s' seems to contain unpushed commits\n", branch)
		return nil
	}

	// A checked out branch must move inside its worktree, or that worktree's index and files go stale.
	dir, checkedOut := s.worktrees[branch]
	switch {
	case !checkedOut:
		if err := gitw.UpdateRef(ref, upstream); err != nil {
			return err
		}
	case !gitw.IsClean(dir):
		fmt.Printf("warning: '%s' is checked out in %s and has local changes or is unavailable, not updated\n", branch, dir)
		return nil
	default:
		if err := gitw.FastForwardIn(dir, upstream); err != nil {
			fmt.Printf("warning: '%s' could not be updated in %s: %s\n", branch, dir, formatError(err))
			return nil
		}
	}

	fmt.Printf("%sUpdated branch %s%s%s (was %s).\n", green, lightGreen, branch, resetColor, shortSHA(diff.A))
	return nil
}

// elsewhere returns the worktree holding branch, if it isn't the current one.
func (s *syncer) elsewhere(branch string) (dir string, ok bool) {
	dir, ok = s.worktrees[branch]
	return dir, ok && branch != s.currentBranch
}

// prune deletes branch if it is merged into the default branch.
func (s *syncer) prune(branch string) error {
	defaultRef := fmt.Sprintf("refs/remotes/%s/%s", s.remote, s.defaultBranch)
	diff, err := gitw.NewRange(localRef(branch), defaultRef)
	if err != nil {
		return err
	}

	if !diff.IsAncestor() {
		fmt.Printf("warning: '%s' was deleted on %s, but appears not merged into '%s'\n", branch, s.remote, s.defaultBranch)
		return nil
	}

	if dir, ok := s.elsewhere(branch); ok {
		fmt.Printf("warning: '%s' was deleted on %s, but is checked out in %s, not deleted\n", branch, s.remote, dir)
		return nil
	}

	// Can't delete the checked out branch.
	if branch == s.currentBranch {
		if dir, ok := s.elsewhere(s.defaultBranch); ok {
			fmt.Printf("warning: '%s' was deleted on %s, but '%s' is checked out in %s, not deleted\n", branch, s.remote, s.defaultBranch, dir)
			return nil
		}
		if err := gitw.Checkout(s.defaultBranch); err != nil {
			return err
		}
		s.currentBranch = s.defaultBranch
		s.worktrees[s.defaultBranch] = s.worktrees[branch]
		delete(s.worktrees, branch)
	}

	if err := gitw.DeleteBranch(branch); err != nil {
		return err
	}

	fmt.Printf("%sDeleted branch %s%s%s (was %s).\n", red, lightRed, branch, resetColor, shortSHA(diff.A))
	return nil
}

func localRef(branch string) string {
	return "refs/heads/" + branch
}

// shortSHA returns the first 7 characters of rev.
func shortSHA(rev string) string {
	if len(rev) <= 7 {
		return rev
	}
	return rev[:7]
}
