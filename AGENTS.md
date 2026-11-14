# AGENTS.md

`git-sync` (module `github.com/arbourd/git-sync`) is a small Go CLI, run as `git sync`, that fast-forwards local branches to match their remotes. With `--prune` it also deletes merged branches whose upstream is gone.

Core logic lives in `sync()` in `main.go`; git plumbing is wrapped in `internal/gitw`.

## Commands

```sh
go build .            # produces ./git-sync binary
go vet ./...
go test -race ./...   # race detector (required by CI)
golangci-lint run     # lint (CI job)
govulncheck ./...     # known vulnerabilities (CI job)
actionlint            # lint .github/workflows; run after changing anything there (local only; not run in CI)
npx --yes --package renovate -- renovate-config-validator --strict   # run after changing .github/renovate.json
```

Don't run `git sync` or `./git-sync` in this repo: it fetches and fast-forwards or deletes its branches. Use the disposable repos described under Testing.

## Layout

```
main.go / main_test.go     CLI entry point — processArgs (flags), sync() (fetch, then fast-forward, warn, or with --prune delete per local branch)
internal/gitw/             thin wrapper around git plumbing commands, built on go-git-cmd-wrapper
internal/gittest/          harness for real, disposable git repos: gittest.NewBare(t) makes a bare remote, .Clone(t) clones it
man/git-sync.1             Man page; shipped in release archives
skills/git-sync/SKILL.md   Agent skill
.github/workflows/ci.yml   CI: test, lint, vulncheck; goreleaser snapshot on branches, release on `v*` tags
.goreleaser.yml            Release archives (LICENSE*, README*, man/*); ldflags `-s -w -X main.Version`
.golangci.yml              golangci-lint config
```

## Data flow

```
main() → processArgs → sync(prune) → Fetch → per branch: upstream() → update() | prune() → IsApplied()
IsApplied: merge-base → no-net-change guard → mergeResult (merge-tree) → patch-ids (rebase, squash)
```

## CLI flags

Parsed by hand in `processArgs`; don't use the stdlib `flag` package. Help and version win over every other argument before `--`; anything else is an error with usage appended. Update the `usage` const in `main.go`, `man/git-sync.1`, and `skills/git-sync/SKILL.md` together, and the skill whenever output or behavior changes too. `git sync --help` is intercepted by git (man page), so `-h` is the in-binary path. Errors print `error: <message>` on stderr and exit 1; any new flag needs cases in `TestProcessArgs`.

## Hard constraints

**Git operations:** use `internal/gitw` (built on `github.com/ldez/go-git-cmd-wrapper/v2`). In non-test code, never use `exec.Command("git", ...)`. The one exception is `gitw.executor`, passed via `g.Executor` when a call needs stdin, extra env, or stdout-only output.

**Error handling:** `fmt.Errorf("context: %w", err)`.

**Fetch:** run `git fetch` with `--no-tags --no-prune-tags`. A force-moved tag (e.g. a rolling "nightly" tag) makes `fetch.pruneTags` hard-fail and aborts the whole sync. See `TestSync_ForceMovedTagIsIgnored` and commit `c67dd55` before changing fetch behavior.

**Worktrees:** never `update-ref` a branch that any worktree has checked out; its index and files go stale and `git status` shows the new commit as staged reversals. `update()` fast-forwards inside the one holding worktree (`gitw.FastForwardIn`) only if it is `gitw.IsClean` (which also fails on a rebase, bisect, merge, cherry-pick or revert in progress), otherwise it warns and continues. A branch held by several worktrees (only possible with `worktree add --force`) is never updated: it warns, since refreshing the others' indexes isn't worth the machinery. Untracked files don't count as dirty, since the merge refuses on a collision. `gitw.Worktrees` maps a branch to a list of paths, counts a worktree stopped mid-rebase or mid-bisect as holding its branch, and relies on `worktree list -z` (git 2.36+, stated in the README and man page; don't add a fallback). If `prune()` switches the current branch, keep `s.worktrees` in step.

**Prune:** deletion is opt-in (`--prune`) and must never lose work. `gitw.IsApplied` treats any failure as "keep". Patch-ids ignore where a change lands, so they only decide when `mergeResult` reports a real conflict; if its `merge-tree --write-tree` check can't run (git older than 2.40, or a `merge` attribute in `$GIT_DIR/info/attributes`) the branch is kept. `mergeResult` must not run user merge drivers, which can hide conflicts: it uses an empty `--attr-source` and no global or system attributes. Add a test in `main_test.go` for any new merge shape, in both the delete and keep directions.

## Testing

- Tests in `main_test.go` exercise `sync()` against **real git repositories**; never mock `gitw`.
- Build a bare remote, clone it once or twice (`local`, `peer`), drive state through `peer`, then call `sync()` from `local`'s working directory (`local.Chdir(t)`).
- Assert on actual ref state (`local.Git(t, "rev-parse", ...)`).
- Repos live in `t.TempDir()` and are isolated from the host's git config via `HOME`/`XDG_CONFIG_HOME`/`GIT_CONFIG_NOSYSTEM`.
- `gittest.Repo.Commit` writes a file named after the message, so messages must be unique and path-safe. Tests that expect a delete must pass `prune=true` to `runSync`. `worktreeSetup` in `main_test.go` builds a clone plus a linked worktree; `addWorktree` adds another on a branch already checked out, and `pushFile` pushes a real file from `peer`.
