# gg: Git with less typing

gg is an alternative command-line interface for [Git][] heavily inspired by
[Mercurial][]. It's designed for less typing in common workflows and make Git
easier to use for both novices and advanced users alike.

-  **Abbreviations.** `git commit -a` takes 13 characters to type. [`gg ci`][]
   takes 5 to do the same thing. Don't worry, you can still type `gg commit`:
   most common operations have shorter aliases.
-  **Built for GitHub and Gerrit.** gg has built-in support for [creating pull
   requests][] and [creating Gerrit changesets][] straight from the
   command-line. No more context switching.
-  **Safer rebases.** [`gg rebase`][] automatically detects common mistakes while
   rebasing and infers the correct change.
-  **Local branches match remote branches.** Using [`gg pull`][] automatically
   creates branches that match your remotes to avoid confusion.
-  **Optional staging.** gg avoids using the staging area to save on typing and
   mistakes. However, gg takes great care to avoid perturbing the staging area,
   so for more advanced commits, you can keep using the same Git commands you're
   used to.
- **Works with existing Git tools.** You can use or not use gg as much as you
  want in your workflow. gg is just a wrapper for the Git CLI, so it works with
  any hooks or custom patches to Git that your project may use. You can see the
  exact Git commands gg runs by passing in `--show-git`.

If you find gg useful, consider [sponsoring @zombiezen][].

[Git]: https://git-scm.com/
[Mercurial]: https://www.mercurial-scm.org/
[creating Gerrit changesets]: https://gg-scm.io/workflow/gerrit/
[creating pull requests]: https://gg-scm.io/workflow/shared/
[`gg ci`]: https://gg-scm.io/cmd/commit/
[`gg pull`]: https://gg-scm.io/cmd/pull/
[`gg rebase`]: https://gg-scm.io/cmd/rebase/
[sponsoring @zombiezen]: https://github.com/sponsors/zombiezen

## Getting Started

Download the [latest release][] from GitHub.  Pre-built binaries are available
for Linux and macOS.

Binary packages are available for Debian-based systems, including Ubuntu.
To use the APT repository:

```
# Import the gg public key
curl -fsSL https://gg-scm.io/apt-key.gpg | sudo apt-key --keyring /etc/apt/trusted.gpg.d/gg.gpg add -

# Add the gg APT repository to the list of sources
echo "deb [signed-by=/etc/apt/trusted.gpg.d/gg.gpg] https://apt.gg-scm.io gg main" | sudo tee /etc/apt/sources.list.d/gg.list
echo "deb-src [signed-by=/etc/apt/trusted.gpg.d/gg.gpg] https://apt.gg-scm.io gg main" | sudo tee -a /etc/apt/sources.list.d/gg.list

# Update the package list and install gg
sudo apt-get update && sudo apt-get install gg
```

To build from source, follow the instructions in [CONTRIBUTING.md][build-source].

You must have a moderately recent copy of Git in your `PATH` to run gg. gg is
tested against Git 2.17.1 and newer. Older versions may work, but are not
supported.

Once you have gg installed in your `PATH`, the [Working Locally][] guide will
show you how to use the basic commands. The [main site][] also includes workflow
guides and reference documentation.

[build-source]: CONTRIBUTING.md#building-from-source
[main site]: https://gg-scm.io/
[latest release]: https://github.com/gg-scm/gg/releases/latest
[Working Locally]: https://gg-scm.io/workflow/local/

## Testimonials

-   "I'm not sure if this is amazing or terrifying.  But it's definitely nifty!" -[@rspier][]

[@rspier]: https://github.com/rspier

## License

Apache 2.0. This is not an official Google product.

gg depends on `golang.org/x/sys`, which is released under a [BSD license][].

[BSD license]: https://go.googlesource.com/sys/+/master/LICENSE
