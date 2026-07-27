# git-sync

`git-sync` (module `github.com/arbourd/git-sync`) is a small Go CLI that fast-forwards or
deletes local branches to match their remote counterparts. Core logic lives in `sync()` in
`main.go`; git plumbing is wrapped in `internal/gitw`.

## Build & test

```console
go build .
go vet ./...
go test ./... -race
```

This mirrors `.github/workflows/ci.yml` exactly (matrix: ubuntu/macOS/windows). Run all three
before considering a change done.

## Layout

- `main.go` — the `sync()` entrypoint: fetches, then walks local branches to fast-forward,
  delete, or warn about each.
- `internal/gitw` — thin wrapper around `git` plumbing commands (rev-parse, for-each-ref,
  merge-base, etc.), built on `github.com/ldez/go-git-cmd-wrapper/v2`.
- `internal/gittest` — test harness for real, disposable git repos. `gittest.NewBare(t)`
  creates a bare remote; `.Clone(t)` clones it. Repos live in `t.TempDir()` and are isolated
  from the host's git config via `HOME`/`XDG_CONFIG_HOME`/`GIT_CONFIG_NOSYSTEM` env vars, so
  tests never touch the real `~/.gitconfig`.

## Testing conventions

Tests in `main_test.go` exercise `sync()` against **real git repositories**, not mocks: build a
bare remote, clone it once or twice (`local`, `peer`), drive state through `peer`, then call
`sync()` from `local`'s working directory (`local.Chdir(t)`) and assert on actual ref state
(`local.Git(t, "rev-parse", ...)`). Follow this pattern for new tests rather than mocking `gitw`.

## Gotchas

- `git fetch` is run with `--no-tags --no-prune-tags` deliberately — fetching/pruning tags
  normally is fine, but a force-moved tag (e.g. a rolling "nightly" release tag) makes
  `fetch.pruneTags` hard-fail and aborts the whole sync. See `TestSync_ForceMovedTagIsIgnored`
  and commit `c67dd55` ("Stop fetching tags") before changing fetch behavior.
- Release is via GoReleaser and only runs in CI on tags matching `v*` (after the `test` job
  passes); it publishes to the `arbourd/homebrew-tap` repo, so it can't be exercised locally.
