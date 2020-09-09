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

Learn more at [gg-scm.io][].

[Git]: https://git-scm.com/
[Mercurial]: https://www.mercurial-scm.org/
[creating Gerrit changesets]: https://gg-scm.io/workflow/gerrit/
[creating pull requests]: https://gg-scm.io/workflow/shared/
[gg-scm.io]: https://gg-scm.io/
[`gg ci`]: https://gg-scm.io/cmd/commit/
[`gg pull`]: https://gg-scm.io/cmd/pull/
[`gg rebase`]: https://gg-scm.io/cmd/rebase/

## Getting Started

Read the [installation guide][] for the most up-to-date information on how to
obtain gg. To build from source, follow the instructions in [CONTRIBUTING.md][build-source].

[build-source]: CONTRIBUTING.md#building-from-source
[installation guide]: https://gg-scm.io/install/

## License

Apache 2.0. This is not an official Google product.

gg depends on `golang.org/x/sys`, which is released under a [BSD license][].

[BSD license]: https://go.googlesource.com/sys/+/master/LICENSE
