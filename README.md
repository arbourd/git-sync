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

## License

`git-sync` is an extraction of the sync command from [github/hub](https://github.com/github/hub) and retains its original MIT license.

## Agent skill

An [agent skill](skills/git-sync/SKILL.md) is available for AI coding agents.
