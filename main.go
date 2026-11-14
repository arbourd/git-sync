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
      --prune    Delete merged branches whose upstream is gone
  -h, --help     Show this help message
  -v, --version  Show version`

func main() {
	opts, err := processArgs(os.Args[1:], os.Stdout)
	if err == nil && !opts.done {
		err = sync(opts.prune)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", formatError(err))
		os.Exit(1)
	}
}

// options are the parsed command line. done means help or the version was
// printed and there is nothing left to do.
type options struct {
	prune bool
	done  bool
}

// processArgs parses args, printing help or the version to stdout if asked.
// Help and version win wherever they appear before "--".
func processArgs(args []string, stdout io.Writer) (opts options, err error) {
	for _, a := range args {
		if a == "--" {
			break
		}
		switch a {
		case "-h", "--help":
			_, err = fmt.Fprintln(stdout, usage)
			return options{done: true}, err
		case "-v", "--version":
			_, err = fmt.Fprintf(stdout, "git-sync %s\n", Version)
			return options{done: true}, err
		}
	}

	// No positional arguments.
	for i, a := range args {
		switch {
		case a == "--prune":
			opts.prune = true
		case a == "--":
			if i+1 < len(args) {
				return options{}, fmt.Errorf("too many arguments: %q\n\n%s", args[i+1], usage)
			}
			return opts, nil
		case strings.HasPrefix(a, "-"):
			return options{}, fmt.Errorf("unknown option %q\n\n%s", a, usage)
		default:
			return options{}, fmt.Errorf("too many arguments: %q\n\n%s", a, usage)
		}
	}
	return opts, nil
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
	dir           string              // root of the current worktree
	worktrees     map[string][]string // branch -> paths of the worktrees holding it
}

func warnf(format string, a ...any) { fmt.Printf("warning: "+format+"\n", a...) }

func sync(prune bool) error {
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

	dir, _ := gitw.Toplevel() // empty in a bare repo, where no worktree is current

	worktrees, err := gitw.Worktrees()
	if err != nil {
		return err
	}

	s := &syncer{
		remote:        remote,
		defaultBranch: defaultBranch,
		currentBranch: gitw.CurrentBranch(),
		dir:           dir,
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

		switch {
		case upstream != "":
			err = s.update(branch, upstream)
		case gone && prune:
			err = s.prune(branch)
		case gone:
			warnf("'%s' was deleted on %s", branch, s.remote)
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
		// Without a configured upstream, there was nothing to lose.
		return upstream, err != nil && gitw.HasMergeConfig(branch)
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
		warnf("'%s' seems to contain unpushed commits", branch)
		return nil
	}

	// A checked out branch must move inside its worktree, or that worktree's index and files go stale.
	// Several worktrees can hold one only through `worktree add --force`, so leave it alone.
	switch dirs := s.worktrees[branch]; {
	case len(dirs) == 0:
		if err := gitw.UpdateRef(ref, upstream); err != nil {
			return err
		}
	case len(dirs) > 1:
		warnf("'%s' is checked out in several worktrees (%s), not updated", branch, strings.Join(dirs, ", "))
		return nil
	case !gitw.IsClean(dirs[0]):
		warnf("'%s' not updated: %s has local changes or an operation in progress", branch, dirs[0])
		return nil
	default:
		if err := gitw.FastForwardIn(dirs[0], upstream); err != nil {
			warnf("'%s' not updated in %s: %s", branch, dirs[0], formatError(err))
			return nil
		}
	}

	fmt.Printf("%sUpdated branch %s%s%s (was %s).\n", green, lightGreen, branch, resetColor, shortSHA(diff.A))
	return nil
}

// elsewhere returns a worktree holding branch, ignoring the current one if branch is checked out there.
func (s *syncer) elsewhere(branch string) (dir string, ok bool) {
	for _, d := range s.worktrees[branch] {
		if branch != s.currentBranch || d != s.dir {
			return d, true
		}
	}
	return "", false
}

// prune deletes branch if it is merged (including rebased or squashed) into the default branch.
func (s *syncer) prune(branch string) error {
	defaultRef := fmt.Sprintf("refs/remotes/%s/%s", s.remote, s.defaultBranch)
	diff, err := gitw.NewRange(localRef(branch), defaultRef)
	if err != nil {
		return err
	}

	if !diff.IsAncestor() && !gitw.IsApplied(localRef(branch), defaultRef) {
		warnf("'%s' was deleted on %s, but appears not merged into '%s'", branch, s.remote, s.defaultBranch)
		return nil
	}

	if dir, ok := s.elsewhere(branch); ok {
		warnf("'%s' was deleted on %s, but is checked out in %s, not deleted", branch, s.remote, dir)
		return nil
	}

	// Can't delete the checked out branch.
	if branch == s.currentBranch {
		if dir, ok := s.elsewhere(s.defaultBranch); ok {
			warnf("'%s' was deleted on %s, but '%s' is checked out in %s, not deleted", branch, s.remote, s.defaultBranch, dir)
			return nil
		}
		if err := gitw.Checkout(s.defaultBranch); err != nil {
			return err
		}
		s.currentBranch = s.defaultBranch
		if s.dir != "" {
			s.worktrees[s.defaultBranch] = []string{s.dir}
		}
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
