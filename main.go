package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/arbourd/git-sync/gitw"
	"github.com/ldez/go-git-cmd-wrapper/v2/branch"
	"github.com/ldez/go-git-cmd-wrapper/v2/checkout"
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
	if gitDir := gitw.IsGitDir(); !gitDir {
		return fmt.Errorf("not a git repository")
	}

	remote := "origin"
	defaultBranch, err := gitw.DefaultBranch(remote)
	if err != nil {
		return err
	}
	fullDefaultBranch := fmt.Sprintf("refs/remotes/%s/%s", remote, defaultBranch)
	currentBranch := gitw.CurrentBranch()

	// git fetch --prune --quiet --progress <remote>
	out, err := git.Fetch(fetch.Quiet, fetch.Prune, fetch.Progress, fetch.Remote(remote))
	if err != nil {
		return fmt.Errorf(out)
	}

	branches, err := gitw.LocalBranches()
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

	branchToRemote, err := gitw.BranchesWithRemotes()
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
		} else if !gitw.HasFile(strings.Split(remoteBranch, "/")...) {
			remoteBranch = ""
		}

		if remoteBranch != "" {
			diff, err := gitw.NewRange(fullBranch, remoteBranch)
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
			diff, err := gitw.NewRange(fullBranch, fullDefaultBranch)
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
