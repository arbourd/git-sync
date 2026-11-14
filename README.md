# git-sync

`git sync` updates all local branches from their remotes.

## Installation

Install with `brew`.

```console
$ brew install arbourd/tap/git-sync
```

## Usage

```console
$ git sync [options]
```

```console
# Fetch, then fast-forward every local branch
$ git sync
Updated branch main (was 8915328).
Updated branch feature-123 (was 7c24329).
```

### Options

| Option             | Description      |
| ------------------ | ---------------- |
| `-h`, `--help`     | Show usage       |
| `-v`, `--version`  | Show the version |
| `--prune`          | Delete merged branches whose upstream is gone |

Delete branches whose upstream is gone and whose work is already in the default branch, even if it was rebased or squashed.

```console
$ git sync --prune
Deleted branch feature-123 (was 7c24329).
```

Without `--prune`, no branch is ever deleted. Requires git 2.40 or newer.

## License

`git-sync` is an extraction of the sync command from [github/hub](https://github.com/github/hub) and retains its original MIT license.

## Agent skill

An [agent skill](skills/git-sync/SKILL.md) is available for AI coding agents.
