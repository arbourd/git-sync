# AGENTS.md

`git-sync` (module `github.com/arbourd/git-sync`) is a small Go CLI, run as `git sync`, that fast-forwards local branches to match their remotes. With `--prune` it also deletes merged branches whose upstream is gone. Core logic lives in `sync()` in `main.go`; git plumbing is wrapped in `internal/gitw`.

## Commands

```sh
go test -race ./...   # race detector (required by CI)
golangci-lint run
govulncheck ./...
actionlint            # run after changing anything in .github/workflows (local only; not run in CI)
mandoc -T lint -W warning man/git-sync.1   # lint the man page (local only; not run in CI)
bunx --package renovate renovate-config-validator --strict   # run after changing .github/renovate.json
```

Try `git sync` only in the disposable repos described under Testing: run in this repo, it fetches and fast-forwards or deletes its branches.

## CLI flags

Parse flags by hand in `processArgs`, not with the stdlib `flag` package. Help and version win over every other argument before `--`; anything else is an error with usage appended. `git sync --help` is intercepted by git (man page), so `-h` is the in-binary path. Errors print `error: <message>` on stderr and exit 1; any new flag needs cases in `TestProcessArgs`.

Update the `usage` const in `main.go`, the options table in `README.md`, `man/git-sync.1`, and `skills/git-sync/SKILL.md` together whenever flags, output, or behavior change.

## Hard constraints

**Git operations:** run git through `internal/gitw` (built on `github.com/ldez/go-git-cmd-wrapper/v2`), not `exec.Command("git", ...)` in non-test code. The one exception is `gitw.executor`, passed via `g.Executor` when a call needs stdin, extra env, or stdout-only output.

**Fetch:** run `git fetch` with `--no-tags --no-prune-tags`. A force-moved tag (e.g. a rolling "nightly" tag) makes `fetch.pruneTags` hard-fail and aborts the whole sync. See commit `c67dd55` before changing fetch behavior.

**Worktrees:**

- Fast-forward a branch held by a worktree inside that worktree (`gitw.FastForwardIn`), not with `update-ref`: that leaves its index stale, so `git status` shows staged reversals.
- Do it only if the worktree is `gitw.IsClean` (no rebase, bisect, merge, cherry-pick or revert in progress either); otherwise warn and continue.
- Leave a branch held by several worktrees alone, with a warning.
- `worktree list -z` needs git 2.36+, stated in the README and man page; there is no fallback.
- If `prune()` switches the current branch, keep `s.worktrees` in step.

**Prune:**

- Deletion is opt-in (`--prune`) and must never lose work: `gitw.IsApplied` treats any failure as "keep".
- Patch-ids only decide when `mergeResult` reports a real conflict, since they ignore where a change lands.
- Keep the branch when the `merge-tree --write-tree` check can't run (git older than 2.40, or a `merge` attribute in `$GIT_DIR/info/attributes`).
- `mergeResult` must not run user merge drivers, which can hide conflicts: use an empty `--attr-source` and no global or system attributes.
- Cover every new merge shape with a test in `main_test.go`, in both the delete and keep directions.

## Testing

- Tests in `main_test.go` exercise `sync()` against **real git repositories**, with `gitw` unmocked.
- `internal/gittest` builds them: make a bare remote, clone it once or twice (`local`, `peer`), drive state through `peer`, then call `sync()` from `local`'s working directory.
- Assert on actual ref state, not output.
- Repos are isolated from the host's git config via `HOME`/`XDG_CONFIG_HOME`/`GIT_CONFIG_NOSYSTEM`.
- `gittest.Repo.Commit` writes a file named after the message, so messages must be unique and path-safe.
- Tests that expect a delete must pass `prune=true` to `runSync`.
