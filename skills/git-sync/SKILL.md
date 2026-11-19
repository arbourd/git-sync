---
name: git-sync
description: Update every local branch from its remote with `git sync`. Use when the user asks to sync, refresh, update, or pull all local branches (not just the current one), or to prune merged branches, even without saying "git sync". `--prune` force-deletes branches, so confirm first.
---

# git-sync

`git sync [--prune]` fetches, then fast-forwards every local branch, instead of checking out and pulling each one.

## Rules

1. **Run plain `git sync` without asking.** It only fast-forwards and warns.
2. **Confirm before `--prune`** unless the user asked to prune or delete merged branches. See Pruning.
3. **Finish with a report:** what was updated or deleted, which warnings need the user's action, and whether the run was partial (see Errors). Warnings are not failures; leave those branches as they are.
4. **Run it inside the repo,** or run `git -C <dir> sync`.

## Recipes

```sh
git sync            # fetch, fast-forward every local branch
git sync --prune    # also delete merged branches whose upstream is gone
```

Output is one line per change, with the commit the branch was at before. It carries ANSI colour codes and warnings go to stdout, so strip the codes before matching text:

```
Updated branch main (was 8915328).
Deleted branch feature-123 (was 7c24329).
warning: 'feature-123' was deleted on origin
warning: 'wip' seems to contain unpushed commits
warning: 'old-idea' was deleted on origin, but appears not merged into 'main'
warning: 'main' not updated: /path/to/worktree has local changes or an operation in progress
warning: 'main' is checked out in several worktrees (/path/a, /path/b), not updated
```

## Pruning

There is no dry run, and deletion is forced (`git branch -D`, no prompt). A branch is deleted only if its upstream is gone and its tip is an ancestor of `<remote>/<default>`, or its changes are already there (rebase- or squash-merged; needs git 2.40+). Anything unproven is kept with a warning.

- A checked-out branch is replaced by the default branch first. A branch checked out in another worktree is kept.
- **Undo:** `git branch <name> <sha>`, using the `was <sha>` from the output.

## Gotchas

- **Remote:** the one whose `<remote>/HEAD` exists, usually `origin`. Without it: `could not find a remote`; fix with `git remote set-head origin -a`. Tags are not fetched or pruned.
- **Upstream:** a branch not tracking that remote is compared with the same-named branch on it, even if it tracks another remote.
- **Unpushed or diverged:** left alone with a warning. Fixing it takes a manual rebase or merge.
- **Worktrees:** a branch checked out in another worktree is fast-forwarded there. It is skipped with a warning if that worktree has changes to tracked files (untracked are ignored) or a rebase, bisect, merge, cherry-pick or revert in progress, or the fast-forward fails. A branch in several worktrees is always skipped.
- **Errors:** they print `error: ...` to stderr and exit 1. The first per-branch error stops the run, so later branches (in ASCII order) are not synced. Known causes: a dirty tree when checking out the default branch to delete the current one, and git older than 2.36, which fails listing worktrees before anything is fetched.
- **One branch:** use `git pull`.

## More

`git sync -h` prints usage; `git sync --help` opens the man page.
