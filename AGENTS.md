# AGENTS.md

`git-sync` (module `github.com/arbourd/git-sync`) is a small Go CLI that fast-forwards or deletes local branches to match their remote counterparts. Invoked as `git sync`. Core logic lives in `sync()` in `main.go`; git plumbing is wrapped in `internal/gitw`.

## Commands

```sh
go build .            # produces ./git-sync binary
go vet ./...
go test -race ./...   # race detector (required by CI)
golangci-lint run     # lint (CI job)
govulncheck ./...     # known vulnerabilities (CI job)
actionlint            # lint .github/workflows; run after changing anything there (local only; not run in CI)
```

## Layout

```
main.go / main_test.go     CLI entry point — processArgs (flags), sync() (fetch, then fast-forward, delete, or warn per local branch)
internal/gitw/             thin wrapper around git plumbing commands, built on go-git-cmd-wrapper
internal/gittest/          harness for real, disposable git repos: gittest.NewBare(t) makes a bare remote, .Clone(t) clones it
man/git-sync.1             Man page; shipped in release archives
.github/workflows/ci.yml   CI: test, lint, vulncheck; goreleaser snapshot on branches, release on `v*` tags
.goreleaser.yml            Release archives (LICENSE*, README*, man/*); ldflags `-s -w -X main.Version`
.golangci.yml              golangci-lint config
```

## CLI flags

Parsed by hand in `processArgs`; don't use the stdlib `flag` package. Help and version win over every other argument before `--`; anything else is an error with usage appended. Update the `usage` const in `main.go` and `man/git-sync.1` together. `git sync --help` is intercepted by git (man page), so `-h` is the in-binary path.

## Hard constraints

**Git operations:** use `internal/gitw` (built on `github.com/ldez/go-git-cmd-wrapper/v2`). In non-test code, never use `exec.Command("git", ...)`.

**Error handling:** `fmt.Errorf("context: %w", err)`.

**Fetch:** run `git fetch` with `--no-tags --no-prune-tags`. A force-moved tag (e.g. a rolling "nightly" tag) makes `fetch.pruneTags` hard-fail and aborts the whole sync. See `TestSync_ForceMovedTagIsIgnored` and commit `c67dd55` before changing fetch behavior.

## Testing

- Tests in `main_test.go` exercise `sync()` against **real git repositories**.
- Build a bare remote, clone it once or twice (`local`, `peer`), drive state through `peer`, then call `sync()` from `local`'s working directory (`local.Chdir(t)`).
- Assert on actual ref state (`local.Git(t, "rev-parse", ...)`).
- Repos live in `t.TempDir()` and are isolated from the host's git config via `HOME`/`XDG_CONFIG_HOME`/`GIT_CONFIG_NOSYSTEM`.

## Do not

- Mock `gitw` in tests; follow the real-repo pattern above.
