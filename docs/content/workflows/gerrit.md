---
title: "Gerrit"
date: 2018-06-13 08:48:58-07:00
weight: 4
---

Some popular open source projects use [Gerrit][], a code review tool, to
manage code contributions. Examples include [Go][], [Android][], and
[Chromium][]. Gerrit has a somewhat unique model for receiving changes that
involves amending commits and pushing to specially formatted ref names.
Developers using Gerrit usually build shortcuts on top of Git to manage this
complexity, but gg has built-in support for Gerrit. gg makes working with
Gerrit as easy as other Git workflows.

[Gerrit]: https://www.gerritcodereview.com/
[Go]: https://golang.org/
[Android]: https://source.android.com/
[Chromium]: https://www.chromium.org/

<!--more-->

## Cloning

Cloning a Gerrit repository is as simple as using the `gg clone` command. You
can install the commit hook that adds the `Change-Id` line to your commit
messages by using `gg gerrithook`.

```shell
gg clone https://example.com/foo.git
cd foo
gg gerrithook
```

## Making Changes

Every Gerrit change should be on a separate branch. `gg branch` will
automatically handle setting the branch's upstream, and `gg mail` automates
pushing the change to the server.

```shell
gg branch myfeature
# hack hack hack
gg commit -m "Added a feature"
gg mail -R foo@example.com
```

To make changes after code review, simply amend the commit on your branch and
run `gg mail` again. (The `-p` flag publishes comments.)

```shell
# hack hack hack
gg commit -amend
gg mail -p
```

### Syncing Your Work

If the upstream branch changed, then you can use `gg rebase` to move your
branch.

```shell
gg pull && gg rebase
```

### Dependent Changes

If you want to send out a sequence of changes that depend on prior changes, keep
the whole sequence on one branch. Running `gg mail` as before will send the
whole chain.

```shell
gg branch myfeature
# hack hack hack
gg commit -m "Added first feature"
# hack hack hack
gg commit -m "Added second feature"
gg mail -R foo@example.com
```

If you want to send a subsequence, you can pass `-r` to `gg mail`:

```shell
# Mails everything in the sequence except the last change.
gg mail -r HEAD~
```

To amend changes earlier in the sequence, use `gg histedit`. `histedit` will
pull open your editor, allowing you to pick the commits you want to edit.

```shell
gg histedit
```

Once you're ready for review, you use `gg mail` as before.

```shell
gg mail -p
```

As your changes get submitted to the upstream branch, you can use `gg evolve` to
rebase your working branch on top of the new commits created for the working
changes. This is usually combined with `gg pull` and `gg rebase` to keep your
branch up-to-date.

```shell
gg pull && gg evolve && gg rebase
```

### Switching Among Changes

You can list all of your branches with:

```shell
gg branch
```

You can use `gg update` to switch to a different branch.

```shell
gg update myfeature
gg update main
```
