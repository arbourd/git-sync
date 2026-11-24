# git-sync

`git sync` updates all local branches from their remotes.

## Installation

Install with `brew`.

```console
$ brew install arbourd/tap/git-sync
```

Install with `go install`.

```console
$ go install github.com/arbourd/git-sync@latest
```

Requires git 2.36 or newer.

## Usage

```console
$ git sync [options]
```

```console
# Fetch, then fast-forward every local branch
$ git sync
Updated branch main (was 8915328).
Updated branch feature-123 (was 7c24329).
warning: 'wip' seems to contain unpushed commits

# Also delete merged branches whose upstream is gone
$ git sync --prune
Deleted branch feature-123 (was 7c24329).

# Sync a repository other than the current directory
$ git -C ~/src/my-repo sync
```

### Options

| Option            | Description                                   |
| ----------------- | --------------------------------------------- |
| `--prune`         | Delete merged branches whose upstream is gone |
| `-h`, `--help`    | Show usage                                    |
| `-v`, `--version` | Show the version                              |
| `--`              | End of options                                |

`git sync --help` is intercepted by git to open the man page, so use `-h` for usage.

### Behavior

`git sync` fetches from the remote whose `<remote>/HEAD` ref exists (usually `origin`), then fast-forwards every local branch that is behind its upstream. A branch not tracking that remote is compared with the same-named branch on it. Tags are neither fetched nor pruned.

Branches with unpushed commits, or that have diverged from their upstream, are left alone with a warning.

A branch checked out in another worktree is fast-forwarded there, unless that worktree has changes to tracked files or a rebase, bisect, merge, cherry-pick or revert in progress. A branch checked out in several worktrees is always left alone.

### Pruning

`--prune` force-deletes a branch whose upstream is gone, but only if its changes are already in the default branch, whether merged, rebased, or squashed (requires git 2.40 or newer). Anything unproven is kept with a warning. There is no dry run.

Restore a deleted branch with `git branch <name> <sha>`, using the `was <sha>` from the output.

## Agent skill

An [agent skill](skills/git-sync/SKILL.md) is included.

## License

`git-sync` is an extraction of the sync command from [github/hub](https://github.com/github/hub) and retains its original MIT license.
