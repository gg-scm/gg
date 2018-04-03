# gg

[![Build Status](https://travis-ci.org/zombiezen/gg.svg?branch=master)][travis]

gg is a wrapper around [Git][] that behaves like [Mercurial][]. It works well enough for
my everyday use.

[Git]: https://git-scm.com/
[Mercurial]: https://www.mercurial-scm.org/
[travis]: https://travis-ci.org/zombiezen/gg

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
  histedit      interactively edit revision history
  init          create a new repository in the given directory
  log           show revision history of entire repository or files
  mail          creates or updates a Gerrit change
  merge         merge another revision into working directory
  pull          pull changes from the specified source
  push          push changes to the specified destination
  rebase        move revision (and descendants) to a different branch
  remove        remove the specified files on the next commit
  revert        restore files to their checkout state
  status        show changed files in the working directory
  update        update working directory (or switch revisions)

options:
  -git path
    	path to git executable
  -show-git
    	log git invocations
  -version
    	display version information
```

## License

Apache 2.0. This is not an official Google product.
