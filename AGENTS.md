# AGENTS.md

`git-sync` is a Go CLI invoked as `git sync`. `sync()` in `main.go` coordinates branch updates; `internal/gitw` wraps git plumbing.

## Commands

```sh
go test -race ./...                                         # required by CI
golangci-lint run
govulncheck ./...
actionlint                                                  # lint .github/workflows (local only; not run in CI)
mandoc -T lint -W warning man/git-sync.1                    # lint the man page (local only; not run in CI)
bunx --package renovate renovate-config-validator --strict  # validate .github/renovate.json
```

Run `git sync` only in disposable test repositories; it fetches and changes branches in the current repository.

## CLI flags

- Before `--`, help and version requests exit 0 and take precedence over other arguments.
- `--` ends option parsing.
- Invalid input prints `error: <message>` plus usage to stderr and exits 1.
- Git intercepts `git sync --help` to open the man page; use `-h` for in-binary help.
- Parse flags by hand in `processArgs`, not with the stdlib `flag` package.
- `git sync` takes no positional arguments; any argument after `--` is an error.
- Add cases to `TestProcessArgs` for every new flag.
- When flags, output, or behavior change, update `usage` in `main.go`, `man/git-sync.1`, `README.md`, and `skills/git-sync/SKILL.md` together.

## Hard constraints

- **Git operations:** use `internal/gitw` for non-test git operations. Its `executor` is the path for calls that need stdin, extra environment, or stdout-only output.
- **Fetch:** retain `--no-tags --no-prune-tags`; force-moved tags can make `fetch.pruneTags` fail and abort sync. Read commit `c67dd55` before changing fetch behavior.
- **Worktrees:**
  - For a branch checked out in a worktree, use `gitw.FastForwardIn` there so its index stays current. Fast-forward only when `gitw.IsClean`; otherwise warn and continue. A branch held by multiple worktrees stays unchanged with a warning.
  - `worktree list -z` requires git 2.36+. Keep that requirement in the README and man page.
  - When `prune()` switches the current branch, update `s.worktrees` too.
- **Prune:**
  - Deletion is opt-in. `gitw.IsApplied` treats every error as "keep" so uncertain cases never lose work.
  - Use patch-ids only after `mergeResult` finds a real conflict; patch-ids ignore where a change lands.
  - If `merge-tree --write-tree` is unavailable (git before 2.40 or a `merge` attribute in `$GIT_DIR/info/attributes`), keep the branch.
  - Prevent user merge drivers from hiding conflicts: `mergeResult` uses an empty `--attr-source` and disables global and system attributes.
  - For each new merge shape, add `main_test.go` coverage for both deletion and retention.

## Testing

- `main_test.go` exercises `sync()` against real repositories with `gitw` unmocked. Use `internal/gittest` to create a bare remote and `local`/`peer` clones, drive changes through `peer`, and run sync from `local`.
- Assert actual ref state. The test repos isolate host git config with `HOME`, `XDG_CONFIG_HOME`, and `GIT_CONFIG_NOSYSTEM`.
- `gittest.Repo.Commit` uses each message as a filename; keep messages unique and path-safe. Pass `prune=true` to `runSync` in tests that expect deletion.
