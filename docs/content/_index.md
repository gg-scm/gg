---
title: "Home"
date: 2018-06-13 08:48:58-07:00
lastmod: 2020-06-22 16:30:00-07:00
---

gg is an alternative command-line interface for [Git][] heavily inspired by
[Mercurial][]. It's designed for less typing in common workflows and make Git
easier to use for both novices and advanced users alike.

[Git]: https://git-scm.com/
[Mercurial]: https://www.mercurial-scm.org/

<!--more-->

-  **Abbreviations.** `git commit -a` takes 13 characters to type. [`gg ci`][]
   takes 5 to do the same thing (don't worry, you can still type `gg commit`).
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

{{< downloadbutton >}}

[creating Gerrit changesets]: {{< relref "workflows/gerrit.md" >}}
[creating pull requests]: {{< relref "workflows/shared.md" >}}
[`gg ci`]: {{< relref "commands/commit.md" >}}
[`gg pull`]: {{< relref "commands/pull.md" >}}
[`gg rebase`]: {{< relref "commands/rebase.md" >}}

## Getting Started

{{< latestrelease >}} Pre-built binaries are available for Linux and macOS.

Binary packages are available for Debian-based systems, including Ubuntu.
To use the APT repository:

```
# Import the gg public key
curl -fsSL https://gg-scm.io/apt-key.gpg | sudo apt-key --keyring /usr/share/keyrings/gg.gpg add -

# Add the gg APT repository to the list of sources
echo "deb [signed-by=/usr/share/keyrings/gg.gpg] https://apt.gg-scm.io gg main" | sudo tee /etc/apt/sources.list.d/gg.list
echo "deb-src [signed-by=/usr/share/keyrings/gg.gpg] https://apt.gg-scm.io gg main" | sudo tee -a /etc/apt/sources.list.d/gg.list

# Update the package list and install gg
sudo apt-get update && sudo apt-get install gg
```

You must have a moderately recent copy of Git in your `PATH` to run gg. gg is
tested against Git 2.17.1 and newer. Older versions may work, but are not
supported.

Once you have gg installed in your `PATH`, the [Working Locally][] guide will
show you how to use the basic commands.

[Working Locally]: {{< relref "workflows/local.md" >}}

## Testimonials

-   "I'm not sure if this is amazing or terrifying.  But it's definitely nifty!" -[@rspier][]

[@rspier]: https://github.com/rspier
