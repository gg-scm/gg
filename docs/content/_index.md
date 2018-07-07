---
title: "Home"
date: 2018-06-13 08:48:58-07:00
lastmod: 2018-07-07 15:35:48-07:00
---

gg is a command-line wrapper around [Git][] that behaves like [Mercurial][]. It
works well enough for the primary author's everyday use. It can be thought of as
an alternate [porcelain][] for Git.

gg is built around three basic principles:

1.  **gg should be good enough to replace the git CLI for common workflows, but
    gg does not replace git.** Asking the user to duck down into git for some
    arcana is perfectly acceptable if the alternative is adding complexity to
    gg.
2.  **Every gg command should map cleanly to a sequence of git commands.** gg's
    implementation might interact with the .git directory directly for
    efficiency, but only if a set of git CLI invocations would produce
    equivalent results.
3.  **Strive for Mercurial's command set, but don't be beholden to it.** For
    example, gg uses git's revision parsing logic instead of trying to replicate
    Mercurial's.  Branches act like Mercurial bookmarks rather than Mercurial's
    branches, since Git doesn't have an equivalent concept.  Simplicity is
    preferred over exact compatibility.

[Git]: https://git-scm.com/
[Mercurial]: https://www.mercurial-scm.org/
[porcelain]: https://git-scm.com/book/en/v2/Git-Internals-Plumbing-and-Porcelain

## Getting Started

{{< latestrelease >}} Binaries are available for Linux and macOS.

You must have a moderately recent copy of Git in your `PATH` to run gg. gg is
tested against Git 2.7.4 and newer. Older versions may work, but are not
supported.

Once you have gg installed in your `PATH`, the [Working Locally][] guide will
show you how to use the basic commands.

[Working Locally]: {{< relref "workflows/local.md" >}}

## Testimonials

-   "I'm not sure if this is amazing or terrifying.  But it's definitely nifty!" -[@rspier][]

[@rspier]: https://github.com/rspier
