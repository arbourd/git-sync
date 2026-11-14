# git-sync

`git sync` updates all local branches from their remotes.

## Installation

Install with `brew`.

```console
$ brew install arbourd/tap/git-sync
```

Requires git 2.36 or newer.

## Usage

```console
$ git sync [options]
```

| Option            | Description                                   |
| ----------------- | --------------------------------------------- |
| `--prune`         | Delete merged branches whose upstream is gone |
| `-h`, `--help`    | Show usage                                    |
| `-v`, `--version` | Show the version                              |

```console
# Fetch, then fast-forward every local branch
$ git sync
Updated branch main (was 8915328).
Updated branch feature-123 (was 7c24329).

# Also delete merged branches whose upstream is gone
$ git sync --prune
Deleted branch feature-123 (was 7c24329).
```

## Agent skill

An [agent skill](skills/git-sync/SKILL.md) is included.

## License

`git-sync` is an extraction of the sync command from [github/hub](https://github.com/github/hub) and retains its original MIT license.
