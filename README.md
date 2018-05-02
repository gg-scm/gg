# gg

[![Build Status](https://travis-ci.org/zombiezen/gg.svg?branch=master)][travis]
[![Coverage Status](https://coveralls.io/repos/github/zombiezen/gg/badge.svg?branch=master)][coveralls]

gg is a wrapper around [Git][] that behaves like [Mercurial][]. It works well enough for
my everyday use. It can be thought of as an alternate [porcelain][] for Git.

[Git]: https://git-scm.com/
[Mercurial]: https://www.mercurial-scm.org/
[travis]: https://travis-ci.org/zombiezen/gg
[coveralls]: https://coveralls.io/github/zombiezen/gg?branch=master
[porcelain]: https://git-scm.com/book/en/v2/Git-Internals-Plumbing-and-Porcelain

## Installing

Download the latest [release][releases] from GitHub.  Binaries are available for
Linux and macOS.

You must have a moderately recent copy of git in your PATH to run gg. gg is
tested against 2.7.4 and newer. Older versions may work, but are not supported.

[releases]: https://github.com/zombiezen/gg/releases

## Building

You must have Go 1.10 or later with [vgo][] to build gg.

```
# From a release tarball:
./build.bash ~/bin/gg

# Or using go tool:
go get -u zombiezen.com/go/gg/cmd/gg
```

[vgo]: https://godoc.org/golang.org/x/vgo

## Using

Use `gg help` to get more help.

```
usage: gg [options] COMMAND [ARG [...]]

Git like Mercurial

basic commands:
  add           add the specified files on the next commit
  branch        list or manage branches
  clone         make a copy of an existing repository
  commit        commit the specified files or all outstanding changes
  diff          diff repository (or selected files)
  init          create a new repository in the given directory
  log           show revision history of entire repository or files
  merge         merge another revision into working directory
  pull          pull changes from the specified source
  push          push changes to the specified destination
  remove        remove the specified files on the next commit
  revert        restore files to their checkout state
  status        show changed files in the working directory
  update        update working directory (or switch revisions)

advanced commands:
  evolve        sync with Gerrit changes in upstream
  gerrithook    install or uninstall Gerrit change ID hook
  histedit      interactively edit revision history
  mail          creates or updates a Gerrit change
  rebase        move revision (and descendants) to a different branch
  upstream      query or set upstream branch

options:
  -git path
    	path to git executable
  -show-git
    	log git invocations
  -version
    	display version information
```

## Testimonials

-   "I'm not sure if this is amazing or terrifying.  But it's definitely nifty!" -[@rspier][]

[@rspier]: https://github.com/rspier

## License

Apache 2.0. This is not an official Google product.

gg depends on `golang.org/x/sys`, which is released under a
[BSD license](https://go.googlesource.com/sys/+/master/LICENSE).
