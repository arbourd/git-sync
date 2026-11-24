---
name: git-sync
description: Sync local branches with `git sync` when the user asks to update, refresh, or pull all branches in a repository. Use `--prune` only on an explicit request to prune or delete merged branches.
---

# git-sync

`git sync [--prune]` fetches from the remote with a default branch and fast-forwards eligible local branches. It leaves diverged branches alone. `--prune` also deletes eligible branches whose upstream was deleted.

## Rules

1. Run plain `git sync` for a sync request. It fast-forwards eligible branches and warns about branches it leaves alone.
2. Use `--prune` only when the user explicitly asks to prune or delete merged branches. Otherwise, explain that pruning deletes branches and ask first. See Pruning.
3. Run in the target repository or use `git -C <dir> sync`.
4. Report branches updated or deleted, warnings that need user action, and whether an error stopped the run early.

## Recipes

```sh
git sync            # fetch, fast-forward every local branch
git sync --prune    # also delete merged branches whose upstream is gone
```

Each changed branch prints its previous commit. Warnings go to stdout. Output includes ANSI color codes.

## Gotchas

- **Remote:** the remote with a `<remote>/HEAD` ref, usually `origin`. If none exists, `git sync` errors; set one with `git remote set-head origin -a`. Tags are not fetched or pruned.
- **Upstream:** a branch not tracking that remote is compared with the same-named branch on it, even if it tracks another remote.
- **Unpushed or diverged:** left alone with a warning. Fixing it takes a manual rebase or merge.
- **Worktrees:** a branch checked out in another worktree is fast-forwarded there. It is skipped with a warning if that worktree has changes to tracked files (untracked are ignored) or a rebase, bisect, merge, cherry-pick or revert in progress, or the fast-forward fails. A branch in several worktrees is always skipped.
- **Errors:** print `error: ...` to stderr and exit 1. An error stops the run, so later branches in ASCII order may remain unsynced. A dirty worktree can prevent pruning the current branch. Git older than 2.36 cannot list worktrees and fails before fetch.
- **One branch:** use `git pull`.

## Pruning

There is no dry run. Deletion is forced with `git branch -D`. A branch is deleted only when its upstream is gone and its tip is an ancestor of `<remote>/<default>`, or its changes are proven present there (including rebase- and squash-merges; requires git 2.40+). Anything unproven is kept with a warning.

- A checked-out branch is replaced by the default branch first. A branch checked out in another worktree is kept.
- **Undo:** `git branch <name> <sha>`, using the `was <sha>` from the output.

## More

`git sync -h` prints usage; `git sync --help` opens the man page.
