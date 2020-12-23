# git-sync

`git-sync` updates all local branches from remotes.

## Installation

Install with `gofish`.

```console
$ gofish rig add https://github.com/arbourd/rig
$ gofish install git-sync
```

Install with `brew`.

```console
$ brew tap arbourd/tap
$ brew install git-sync
```

Install with `go get`.

```console
$ go get -u github.com/arbourd/git-sync
```

## Usage

Update your branches.

```console
$ git sync
Updated branch main (was 8915328).
Updated branch feature-123 (was 7c24329).
```

## License

`git-sync` is an extraction of the sync command from [github/hub](https://github.com/github/hub) and retains its original MIT license.
