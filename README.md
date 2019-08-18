# gg

gg is a wrapper around [Git][] that behaves like [Mercurial][]. It works well enough for
my everyday use. It can be thought of as an alternate [porcelain][] for Git.

[Git]: https://git-scm.com/
[Mercurial]: https://www.mercurial-scm.org/
[porcelain]: https://git-scm.com/book/en/v2/Git-Internals-Plumbing-and-Porcelain

## Getting Started

Download the [latest release][] from GitHub.  Binaries are available for
Linux and macOS.

To build from source, follow the instructions in [CONTRIBUTING.md][build-source].

You must have a moderately recent copy of Git in your `PATH` to run gg. gg is
tested against Git 2.11.0 and newer. Older versions may work, but are not
supported.

Once you have gg installed in your `PATH`, the [Working Locally][] guide will
show you how to use the basic commands. The [main site][] also includes workflow
guides and reference documentation.

[build-source]: CONTRIBUTING.md#building-from-source
[main site]: https://gg-scm.io/
[latest release]: https://github.com/zombiezen/gg/releases/latest
[Working Locally]: https://gg-scm.io/workflow/local/

## Testimonials

-   "I'm not sure if this is amazing or terrifying.  But it's definitely nifty!" -[@rspier][]

[@rspier]: https://github.com/rspier

## License

Apache 2.0. This is not an official Google product.

gg depends on `golang.org/x/sys`, which is released under a [BSD license][].

[BSD license]: https://go.googlesource.com/sys/+/master/LICENSE
