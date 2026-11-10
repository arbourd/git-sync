# git-sync

`git-sync` (module `github.com/arbourd/git-sync`) is a small Go CLI that fast-forwards or
deletes local branches to match their remote counterparts. Core logic lives in `sync()` in
`main.go`; git plumbing is wrapped in `internal/gitw`.

## Build & test

```console
go build .
go vet ./...
go test ./... -race
golangci-lint run
govulncheck ./...
actionlint
```

The first three mirror the `test` job in `.github/workflows/ci.yml` (ubuntu/macOS/windows), which
also runs `go test -v` and `./git-sync --version`. `golangci-lint` and `govulncheck` mirror the
`lint` and `vulncheck` jobs; `actionlint` checks the workflows. Run all before considering a change done.

## Layout

- `main.go` — `processArgs` (flags) and `sync()`: fetches, then walks local branches to
  fast-forward, delete, or warn about each.
- `internal/gitw` — thin wrapper around `git` plumbing commands, built on
  `github.com/ldez/go-git-cmd-wrapper/v2`.
- `internal/gittest` — test harness for real, disposable git repos. `gittest.NewBare(t)`
  creates a bare remote; `.Clone(t)` clones it. Repos live in `t.TempDir()` and are isolated
  from the host's git config via `HOME`/`XDG_CONFIG_HOME`/`GIT_CONFIG_NOSYSTEM`.
- `.github/workflows/ci.yml` — jobs: test, lint, vulncheck; `snapshot` (GoReleaser snapshot, non-tag
  refs, read-only token); `release` on `v*` tags (`contents: write`). Publishing can't be exercised locally.
- `.goreleaser.yml` — archives ship `LICENSE*`, `README*`, `man/*`; ldflags `-s -w -X main.Version`.

## Flags

Flags are parsed by hand in `processArgs`; don't use the stdlib `flag` package. Help and version win
over every other argument before `--`; anything else is an error with usage appended. Update the
`usage` const in `main.go` and `man/git-sync.1` together. `git sync --help` is intercepted by git
(man page), so `-h` is the in-binary path.

## Testing conventions

Tests in `main_test.go` exercise `sync()` against **real git repositories**, not mocks: build a
bare remote, clone it once or twice (`local`, `peer`), drive state through `peer`, then call
`sync()` from `local`'s working directory (`local.Chdir(t)`) and assert on actual ref state
(`local.Git(t, "rev-parse", ...)`). Follow this pattern for new tests rather than mocking `gitw`.

## Gotchas

- `git fetch` is run with `--no-tags --no-prune-tags` deliberately: a force-moved tag (e.g. a
  rolling "nightly" tag) makes `fetch.pruneTags` hard-fail and aborts the whole sync. See
  `TestSync_ForceMovedTagIsIgnored` and commit `c67dd55` before changing fetch behavior.
