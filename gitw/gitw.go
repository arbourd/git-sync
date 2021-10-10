package gitw

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ldez/go-git-cmd-wrapper/v2/branch"
	"github.com/ldez/go-git-cmd-wrapper/v2/config"
	"github.com/ldez/go-git-cmd-wrapper/v2/git"
	"github.com/ldez/go-git-cmd-wrapper/v2/revparse"
	"github.com/ldez/go-git-cmd-wrapper/v2/types"
)

func NewRange(a, b string) (*Range, error) {
	// git rev-parse --quiet <a> <b>
	out, err := git.RevParse(revparse.Quiet, revparse.Args(a, b))
	if err != nil {
		return nil, fmt.Errorf(out)
	}

	lines := outputLines(out)
	if len(lines) != 2 {
		return nil, fmt.Errorf("cannot parse range %s..%s", a, b)
	}

	return &Range{lines[0], lines[1]}, nil
}

type Range struct {
	A string
	B string
}

func (r *Range) IsIdentical() bool {
	return strings.EqualFold(r.A, r.B)
}

func (r *Range) IsAncestor() bool {
	// git merge-base --is-ancestor <r.A> <r.B>
	_, err := git.Raw("merge-base", func(g *types.Cmd) {
		g.AddOptions("--is-ancestor")
		g.AddOptions(r.A)
		g.AddOptions(r.B)
	})

	if err != nil {
		return false
	}
	return true
}

func HasFile(segments ...string) bool {
	// The blessed way to resolve paths within git dir since Git 2.5.0
	// pathCmd := gitCmd("rev-parse", "-q", "--git-path", filepath.Join(segments...))
	out, err := git.RevParse(revparse.Quiet, revparse.GitPath(filepath.Join(segments...)))
	if err != nil {
		return false
	}

	lines := outputLines(out)
	if len(lines) != 1 {
		return false
	}

	if _, err := os.Stat(lines[0]); err != nil {
		return false
	}
	return true
}

// RemoteFromHead gets the remote where the HEAD is.
func RemoteFromHead() (string, error) {
	// git branch -r
	remotes, err := git.Branch(branch.Remotes)
	if err != nil {
		return "", err
	}

	r := regexp.MustCompile(`([\w\d]+)\/HEAD`)
	match := r.FindAllStringSubmatch(remotes, -1)

	if len(match) < 1 || len(match[0]) < 2 {
		return "", fmt.Errorf("could not find a remote")
	}
	return match[0][1], nil
}

// BranchesWithRemotes uses the Git config to determine which branches also exist on the remotes.
// Returns a list of branches.
func BranchesWithRemotes() (map[string]string, error) {
	// git config --get-regexp 'branch.*.remote'
	out, err := git.Config(config.GetRegexp("branch.*.remote", ""))
	if err != nil {
		return map[string]string{}, fmt.Errorf(out)
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
		return []string{}, fmt.Errorf(out)
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
		return "", fmt.Errorf(out)
	}

	branch := strings.Replace(strings.TrimSpace(out), fmt.Sprintf("refs/remotes/%s/", remote), "", 1)
	return branch, nil
}

// IsGitDir checks if the current working directory containers a .git folder.
func IsGitDir() bool {
	// git rev-parse --git-dir
	_, err := git.RevParse(revparse.GitDir)
	if err != nil {
		return false
	}

	return true
}

func outputLines(output string) []string {
	output = strings.TrimSuffix(output, "\n")
	if output == "" {
		return []string{}
	}
	return strings.Split(output, "\n")
}
