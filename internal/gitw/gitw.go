package gitw

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/ldez/go-git-cmd-wrapper/v2/branch"
	"github.com/ldez/go-git-cmd-wrapper/v2/checkout"
	"github.com/ldez/go-git-cmd-wrapper/v2/config"
	"github.com/ldez/go-git-cmd-wrapper/v2/fetch"
	"github.com/ldez/go-git-cmd-wrapper/v2/git"
	"github.com/ldez/go-git-cmd-wrapper/v2/global"
	"github.com/ldez/go-git-cmd-wrapper/v2/merge"
	"github.com/ldez/go-git-cmd-wrapper/v2/revparse"
	"github.com/ldez/go-git-cmd-wrapper/v2/status"
	"github.com/ldez/go-git-cmd-wrapper/v2/types"
	"github.com/ldez/go-git-cmd-wrapper/v2/worktree"
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

// IsApplied reports whether branch was rebased or squash merged into upstream.
//
// It errs towards false: a branch with no net change, or whose work can't be
// matched to upstream, is kept. It requires git >= 2.40.
func IsApplied(branch, upstream string) bool {
	base, err := git.Raw("merge-base", func(g *types.Cmd) {
		g.AddOptions(upstream)
		g.AddOptions(branch)
	})
	if err != nil {
		return false
	}
	base = strings.TrimSpace(base)

	// No net change (such as add then revert): nothing here was applied.
	_, err = git.Raw("diff", func(g *types.Cmd) {
		g.AddOptions("--quiet")
		g.AddOptions(base)
		g.AddOptions(branch)
	})
	if err == nil {
		return false
	}

	if mergesToUpstreamTree(branch, upstream) {
		return true
	}

	// The patch checks walk all history since the merge-base, so they go last.
	ups, err := patchIDs("log", "-p", "--no-merges", base+".."+upstream)
	if err != nil {
		return false
	}

	// A rebase gives every commit of the branch an identical patch upstream.
	// Merge commits have no patch of their own, so only branches without any qualify.
	merges, err := git.Raw("rev-list", func(g *types.Cmd) {
		g.AddOptions("--merges")
		g.AddOptions(upstream + ".." + branch)
	})
	if err == nil && strings.TrimSpace(merges) == "" {
		ids, err := patchIDs("log", "-p", "--no-merges", base+".."+branch)
		if err == nil && len(ids) > 0 && !slices.ContainsFunc(ids, func(id string) bool { return !slices.Contains(ups, id) }) {
			return true
		}
	}

	// A squash commit has the same patch as the branch's net diff, even if later
	// commits edited the same files.
	if net, err := patchIDs("diff", base, branch); err == nil && len(net) == 1 && slices.Contains(ups, net[0]) {
		return true
	}
	return false
}

// mergesToUpstreamTree reports whether merging branch into upstream yields upstream's own tree,
// which means the merge changes nothing.
//
// The empty tree as attribute source and no global attributes file keep
// merge drivers from resolving conflicts that would otherwise block it.
// $GIT_DIR/info/attributes can't be turned off, so the check is skipped when it exists.
func mergesToUpstreamTree(branch, upstream string) bool {
	if hasInfoAttributes() {
		return false
	}

	// git --attr-source=<empty tree> merge-tree --write-tree <upstream> <branch>
	out, err := git.Raw("merge-tree", func(g *types.Cmd) {
		g.AddBaseOptions("--attr-source=" + emptyTree)
		g.AddBaseOptions("-c")
		g.AddBaseOptions("core.attributesFile=" + os.DevNull)
		g.AddOptions("--write-tree")
		g.AddOptions(upstream)
		g.AddOptions(branch)
		g.Executor = executor("", "GIT_ATTR_NOSYSTEM=1")
	})
	if err != nil {
		return false
	}
	tree, err := git.RevParse(revparse.Quiet, revparse.Args(upstream+"^{tree}"))
	if err != nil {
		return false
	}
	first, _, _ := strings.Cut(out, "\n")
	return strings.TrimSpace(first) == strings.TrimSpace(tree)
}

// executor runs git with stdin and extra env, which the default executor can't.
// It returns stdout only, so stderr warnings can't end up in diffs or tree ids.
func executor(stdin string, env ...string) types.Executor {
	return func(ctx context.Context, name string, _ bool, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdin = strings.NewReader(stdin)
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.Output()
		return string(out), err
	}
}

// hasInfoAttributes reports whether the repository has a non-empty $GIT_DIR/info/attributes.
// When unsure it says yes.
func hasInfoAttributes() bool {
	// git rev-parse --git-path info/attributes
	path, err := git.RevParse(revparse.GitPath("info/attributes"))
	if err != nil {
		return true
	}
	fi, err := os.Stat(strings.TrimSpace(path))
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	return err != nil || fi.Size() > 0
}

// emptyTree is the well-known object id of the empty tree, which git always resolves.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// patchIDs returns the verbatim patch-id of each patch in the output of git cmd args.
// Whitespace counts, so patches that differ only in whitespace don't match.
//
// shortcut: buffers the whole diff in memory, stream it if huge histories matter.
func patchIDs(cmd string, args ...string) ([]string, error) {
	diff, err := git.Raw(cmd, func(g *types.Cmd) {
		for _, a := range slices.Concat([]string{"--no-ext-diff", "--no-textconv", "--no-color"}, args) {
			g.AddOptions(a)
		}
		g.Executor = executor("")
	})
	if err != nil {
		return nil, err
	}
	out, err := git.Raw("patch-id", func(g *types.Cmd) {
		g.AddOptions("--verbatim")
		g.Executor = executor(diff)
	})
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, l := range outputLines(out) {
		if f := strings.Fields(l); len(f) > 0 {
			ids = append(ids, f[0])
		}
	}
	return ids, nil
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

// Fetch fetches and prunes remote without touching tags.
//
// Tags are skipped because a force-moved tag makes fetch.pruneTags hard-fail.
func Fetch(remote string) error {
	// git fetch --prune --progress --no-tags --no-prune-tags <remote>
	noPruneTags := func(g *types.Cmd) { g.AddOptions("--no-prune-tags") }
	out, err := git.Fetch(fetch.Prune, fetch.Progress, fetch.NoTags, noPruneTags, fetch.Remote(remote))
	if err != nil {
		return errors.New(out)
	}
	return nil
}

// Upstream returns the full upstream ref of branch (like refs/remotes/origin/main).
// It errors if the branch has no upstream, including when it was deleted on the remote.
func Upstream(branch string) (string, error) {
	// git rev-parse --symbolic-full-name <branch>@{upstream}
	out, err := git.RevParse(revparse.SymbolicFullName, revparse.Args(branch+"@{upstream}"))
	if err != nil {
		return "", errors.New(out)
	}
	return strings.TrimSpace(out), nil
}

// HasMergeConfig reports whether branch has an upstream configured (branch.<name>.merge).
func HasMergeConfig(branch string) bool {
	// git config --get branch.<name>.merge
	_, err := git.Raw("config", func(g *types.Cmd) {
		g.AddOptions("--get")
		g.AddOptions("branch." + branch + ".merge")
	})
	return err == nil
}

// Worktrees maps each checked out branch to its worktree path.
func Worktrees() (map[string]string, error) {
	// git worktree list --porcelain
	out, err := git.Worktree(worktree.List, worktree.Porcelain)
	if err != nil {
		return nil, errors.New(out)
	}

	trees := map[string]string{}
	var path string
	for _, line := range outputLines(out) {
		key, val, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			path = val
		case "branch":
			if b, ok := strings.CutPrefix(val, "refs/heads/"); ok {
				trees[b] = path
			}
		}
	}
	return trees, nil
}

// IsClean reports whether the worktree at dir has no changes to tracked files (false if unreadable).
// Untracked files are ignored: a fast-forward only fails on those if it would overwrite one.
func IsClean(dir string) bool {
	// git -C <dir> --no-optional-locks status --porcelain --untracked-files=no
	// No optional locks: don't contend for index.lock with whatever is running in dir.
	out, err := git.Status(global.UpperC(dir), global.NoOptionalLocks, status.Porcelain(""), status.UntrackedFiles("no"))
	return err == nil && strings.TrimSpace(out) == ""
}

// FastForwardIn fast-forwards the worktree at dir to ref.
func FastForwardIn(dir, ref string) error {
	// git -C <dir> merge --ff-only --quiet <ref>
	out, err := git.Merge(global.UpperC(dir), merge.FfOnly, merge.Quiet, merge.Commits(ref))
	if err != nil {
		return errors.New(out)
	}
	return nil
}

// UpdateRef points ref at target.
func UpdateRef(ref, target string) error {
	// git update-ref <ref> <target>
	out, err := git.Raw("update-ref", func(g *types.Cmd) {
		g.AddOptions(ref)
		g.AddOptions(target)
	})
	if err != nil {
		return errors.New(out)
	}
	return nil
}

// Checkout switches to the named branch.
func Checkout(name string) error {
	// git checkout --quiet <name>
	out, err := git.Checkout(checkout.Quiet, checkout.Branch(name))
	if err != nil {
		return errors.New(out)
	}
	return nil
}

// DeleteBranch force-deletes the named local branch.
func DeleteBranch(name string) error {
	// git branch -D <name>
	out, err := git.Branch(branch.DeleteForce, branch.BranchName(name))
	if err != nil {
		return errors.New(out)
	}
	return nil
}

func outputLines(output string) []string {
	output = strings.TrimSuffix(output, "\n")
	if output == "" {
		return []string{}
	}
	return strings.Split(output, "\n")
}
