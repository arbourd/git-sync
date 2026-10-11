package main

import (
	"errors"
	"fmt"
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

func main() {
	if err := sync(); err != nil {
		fmt.Printf("fatal: %s\n", formatError(err))
		os.Exit(1)
	}
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

	s := &syncer{
		remote:        remote,
		defaultBranch: defaultBranch,
		currentBranch: gitw.CurrentBranch(),
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

	if branch == s.currentBranch {
		err = gitw.FastForwardCurrent(upstream)
	} else {
		err = gitw.UpdateRef(ref, upstream)
	}
	if err != nil {
		return err
	}

	fmt.Printf("%sUpdated branch %s%s%s (was %s).\n", green, lightGreen, branch, resetColor, shortSHA(diff.A))
	return nil
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

	// Can't delete the checked out branch.
	if branch == s.currentBranch {
		if err := gitw.Checkout(s.defaultBranch); err != nil {
			return err
		}
		s.currentBranch = s.defaultBranch
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
