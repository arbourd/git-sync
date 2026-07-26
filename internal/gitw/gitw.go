package gitw

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/ldez/go-git-cmd-wrapper/v2/branch"
	"github.com/ldez/go-git-cmd-wrapper/v2/config"
	"github.com/ldez/go-git-cmd-wrapper/v2/git"
	"github.com/ldez/go-git-cmd-wrapper/v2/revparse"
	"github.com/ldez/go-git-cmd-wrapper/v2/types"
)

// NewRange resolves a and b to a Range.
func NewRange(a, b string) (*Range, error) {
	// git rev-parse --quiet <a> <b>
	out, err := git.RevParse(revparse.Quiet, revparse.Args(a, b))
	if err != nil {
		return nil, errors.New(out)
	}

	lines := outputLines(out)
	if len(lines) != 2 {
		return nil, fmt.Errorf("cannot parse range %s..%s", a, b)
	}

	return &Range{lines[0], lines[1]}, nil
}

// Range is a pair of resolved revisions.
type Range struct {
	A string
	B string
}

// IsIdentical reports whether A and B are the same revision.
func (r *Range) IsIdentical() bool {
	return strings.EqualFold(r.A, r.B)
}

// IsAncestor reports whether A is an ancestor of B.
func (r *Range) IsAncestor() bool {
	// git merge-base --is-ancestor <r.A> <r.B>
	_, err := git.Raw("merge-base", func(g *types.Cmd) {
		g.AddOptions("--is-ancestor")
		g.AddOptions(r.A)
		g.AddOptions(r.B)
	})
	return err == nil
}

// HasFile reports whether the ref named by joining segments with "/" exists.
func HasFile(segments ...string) bool {
	ref := strings.Join(segments, "/")

	// git rev-parse --quiet --verify <ref>
	_, err := git.RevParse(revparse.Quiet, revparse.Verify, revparse.Args(ref))
	return err == nil
}

// RemoteFromHead gets the remote where the HEAD is.
func RemoteFromHead() (string, error) {
	// git branch -r
	remotes, err := git.Branch(branch.Remotes)
	if err != nil {
		return "", err
	}

	r := regexp.MustCompile(`([\w\d]+)\/HEAD`)
	match := r.FindStringSubmatch(remotes)

	if len(match) < 2 {
		return "", fmt.Errorf("could not find a remote")
	}
	return match[1], nil
}

// BranchesWithRemotes returns a map of local branches to their remote.
func BranchesWithRemotes() (map[string]string, error) {
	// git config --get-regexp 'branch.*.remote'
	out, err := git.Config(config.GetRegexp("branch.*.remote", ""))
	if err != nil {
		return map[string]string{}, errors.New(out)
	}
	lines := outputLines(out)

	branchToRemote := map[string]string{}
	configRe := regexp.MustCompile(`^branch\.(.+?)\.remote (.+)`)
	for _, line := range lines {
		if matches := configRe.FindStringSubmatch(line); len(matches) > 0 {
			branchToRemote[matches[1]] = matches[2]
		}
	}

	return branchToRemote, nil
}

// LocalBranches gets a list of all local branches as a slice of strings.
func LocalBranches() ([]string, error) {
	// git for-each-ref --format='%(refname:short)' refs/heads/
	out, err := git.Raw("for-each-ref", func(g *types.Cmd) {
		g.AddOptions("--format='%(refname:short)'")
		g.AddOptions("refs/heads/")
	})
	if err != nil {
		return []string{}, errors.New(out)
	}

	lines := outputLines(out)
	for i := range lines {
		lines[i] = strings.Trim(lines[i], "'")
	}

	return lines, nil
}

// CurrentBranch gets the current local branch.
func CurrentBranch() string {
	// git rev-parse --abbrev-ref HEAD
	out, err := git.RevParse(revparse.AbbrevRef(""), revparse.Args("HEAD"))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(out)
}

// DefaultBranch gets the default branch (like main) from the remote refs.
func DefaultBranch(remote string) (string, error) {
	ref := fmt.Sprintf("refs/remotes/%s/HEAD", remote)

	// git symbolic-ref refs/remotes/<remote>/HEAD
	out, err := git.Raw("symbolic-ref", func(g *types.Cmd) {
		g.AddOptions(ref)
	})
	if err != nil {
		return "", errors.New(out)
	}

	branch := strings.Replace(strings.TrimSpace(out), fmt.Sprintf("refs/remotes/%s/", remote), "", 1)
	return branch, nil
}

// IsGitDir checks if the current working directory containers a .git folder.
func IsGitDir() bool {
	// git rev-parse --git-dir
	_, err := git.RevParse(revparse.GitDir)
	return err == nil
}

func outputLines(output string) []string {
	output = strings.TrimSuffix(output, "\n")
	if output == "" {
		return []string{}
	}
	return strings.Split(output, "\n")
}
