---
title: Shared Repository
date: 2018-07-07 08:21:47 -07:00
weight: 2
---

gg supports the [shared repository model][models] of development used by
individuals and smaller teams. In this workflow, the source of truth is a single
shared repository. Each developer clones to a repository on their own machine
and makes their changes. When they are ready to share, they push to a branch on
the single shared repository. If they are using pull requests, each change goes
onto a [branch][], which then gets reviewed. Otherwise, usually commits go
directly to the default branch (usually `master`).

[models]: https://help.github.com/articles/about-collaborative-development-models/
[branch]: https://help.github.com/articles/about-branches/

## Cloning

When setting up your working copy, you will first clone the repository.

```shell
gg clone https://example.com/foo.git
cd foo
```

Replace the URL as needed.

## Making Changes Directly to the Default Branch

If you are working by yourself or your team does not use pull requests, you will
typically make changes directly to the default branch (usually `master`).

```shell
# hack hack hack
gg commit -m "Added a feature"
gg push
```

If the push fails because someone else pushed commits while you were working,
you can rebase your commits on top of the new commits.

```shell
gg pull && gg rebase
```

## Making Changes on a Feature Branch

In a pull request workflow, every change should be on a separate branch. `gg
branch` will automatically handle setting the branch's upstream, which is used
for determining the default branch for merges and rebases.

```shell
gg branch myfeature
# hack hack hack
gg commit -m "Added a feature"
gg push
```

To make changes after code review, simply push more commits to your branch and
run `gg push` again.

```shell
# hack hack hack
gg commit -m "Addressed code review comments"
gg push
```

### Syncing Your Work with the Upstream Branch

If the upstream branch (usually `master`) changed, then you can use `gg merge`
to merge in commits.

```shell
gg pull && gg merge
```

If there are no conflicts or test breakages, you can run `gg commit` to commit
the merge.

### Syncing Your Work with your Feature Branch

If your feature branch changes (for example, if another team member adds
commits), then first you need to download the new commits from your fork using
`gg pull`.

```shell
gg pull -r CURRENT_BRANCH
```

Replace `CURRENT_BRANCH` with the name of your current branch.

Once you've downloaded the commits, you will need to either merge or rebase your
local commits. Merging will create a new commit that merges the two streams of
work, whereas rebasing will recreate your changes on top of the downloaded
commits.

To create a merge commit:

```shell
gg merge FETCH_HEAD
# Resolve any conflicts, run tests.
gg commit
```

Or to rebase your commits onto the downloaded changes:

```shell
gg rebase -base=FETCH_HEAD -dst=FETCH_HEAD
```

### Switching Among Changes

You can list all of your branches with:

```shell
gg branch
```

You can use `gg update` to switch to a different branch.

```shell
gg update myfeature
gg update master
```
