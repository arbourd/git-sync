package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ldez/go-git-cmd-wrapper/v2/branch"
	"github.com/ldez/go-git-cmd-wrapper/v2/checkout"
	"github.com/ldez/go-git-cmd-wrapper/v2/config"
	"github.com/ldez/go-git-cmd-wrapper/v2/fetch"
	"github.com/ldez/go-git-cmd-wrapper/v2/git"
	"github.com/ldez/go-git-cmd-wrapper/v2/merge"
	"github.com/ldez/go-git-cmd-wrapper/v2/revparse"
	"github.com/ldez/go-git-cmd-wrapper/v2/types"
)

func main() {
	err := sync()
	if err != nil {
		fmt.Printf("fatal: %s\n", strings.TrimPrefix(strings.TrimSpace(err.Error()), "fatal: "))
		os.Exit(1)
	}
}

func sync() error {
	if gitDir := isGitDir(); !gitDir {
		return fmt.Errorf("not a git repository")
	}

	remote := "origin"
	defaultBranch, err := defaultBranch(remote)
	if err != nil {
		return err
	}
	fullDefaultBranch := fmt.Sprintf("refs/remotes/%s/%s", remote, defaultBranch)
	currentBranch := currentBranch()

	// git fetch --prune --quiet --progress <remote>
	out, err := git.Fetch(fetch.Quiet, fetch.Prune, fetch.Progress, fetch.Remote(remote))
	if err != nil {
		return fmt.Errorf(out)
	}

	branches, err := localBranches()
	if err != nil {
		return err
	}

	var green,
		lightGreen,
		red,
		lightRed,
		resetColor string

	colorize := true
	if colorize {
		green = "\033[32m"
		lightGreen = "\033[32;1m"
		red = "\033[31m"
		lightRed = "\033[31;1m"
		resetColor = "\033[0m"
	}

	branchToRemote, err := branchesWithRemotes()
	if err != nil {
		return err
	}

	for _, wbranch := range branches {
		fullBranch := fmt.Sprintf("refs/heads/%s", wbranch)
		remoteBranch := fmt.Sprintf("refs/remotes/%s/%s", remote, wbranch)
		gone := false

		if branchToRemote[wbranch] == remote {
			if upstream, err := git.RevParse(revparse.SymbolicFullName, revparse.Args(fmt.Sprintf("%s@{upstream}", wbranch))); err == nil {
				remoteBranch = strings.TrimSpace(upstream)
			} else {
				remoteBranch = ""
				gone = true
			}
		} else if !hasFile(strings.Split(remoteBranch, "/")...) {
			remoteBranch = ""
		}

		if remoteBranch != "" {
			diff, err := newRange(fullBranch, remoteBranch)
			if err != nil {
				return err
			}

			if diff.IsIdentical() {
				continue
			} else if diff.IsAncestor() {
				if wbranch == currentBranch {
					// git merge --ff-only --quiet <remoteBranch>
					out, err := git.Merge(merge.FfOnly, merge.Quiet, merge.Commits(remoteBranch))
					if err != nil {
						return fmt.Errorf(out)
					}
				} else {
					// git update-ref <fullBranch> <remoteBranch>
					out, err := git.Raw("update-ref", func(g *types.Cmd) {
						g.AddOptions(fullBranch)
						g.AddOptions(remoteBranch)
					})
					if err != nil {
						return fmt.Errorf(out)
					}
				}
				fmt.Printf("%sUpdated branch %s%s%s (was %s).\n", green, lightGreen, wbranch, resetColor, diff.A[0:7])
			} else {
				fmt.Printf("warning: '%s' seems to contain unpushed commits\n", wbranch)
			}
		} else if gone {
			diff, err := newRange(fullBranch, fullDefaultBranch)
			if err != nil {
				return err
			}

			if diff.IsAncestor() {
				if wbranch == currentBranch {
					// git checkout --quiet <defaultBranch>
					out, err := git.Checkout(checkout.Quiet, checkout.Branch(defaultBranch))
					if err != nil {
						return fmt.Errorf(out)
					}
					currentBranch = defaultBranch
				}

				// git branch -D <wbranch>
				out, err := git.Branch(branch.Delete, branch.BranchName(wbranch))
				if err != nil {
					return fmt.Errorf(out)
				}
				fmt.Printf("%sDeleted branch %s%s%s (was %s).\n", red, lightRed, wbranch, resetColor, diff.A[0:7])
			} else {
				fmt.Printf("warning: '%s' was deleted on %s, but appears not merged into '%s'\n", wbranch, remote, defaultBranch)
			}
		}
	}

	return nil
}

func newRange(a, b string) (*Range, error) {
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

func hasFile(segments ...string) bool {
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

// branchesWithRemotes uses the Git config to determine which branches also exist on the remotes.
// Returns a list of branches.
func branchesWithRemotes() (map[string]string, error) {
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

// localBranches gets a list of all local branches as a slice of strings.
func localBranches() ([]string, error) {
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

// currentBranch gets the current local branch.
func currentBranch() string {
	// git rev-parse --abbrev-ref HEAD
	out, err := git.RevParse(revparse.AbbrevRef(""), revparse.Args("HEAD"))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(out)
}

// defaultBranch gets the default branch (like main) from the remote refs.
func defaultBranch(remote string) (string, error) {
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

func outputLines(output string) []string {
	output = strings.TrimSuffix(output, "\n")
	if output == "" {
		return []string{}
	}
	return strings.Split(output, "\n")
}

// isGitDir checks if the current working directory containers a .git folder.
func isGitDir() bool {
	// git rev-parse --git-dir
	_, err := git.RevParse(revparse.GitDir)
	if err != nil {
		return false
	}

	return true
}
