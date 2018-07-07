---
title: Fork and Pull
date: 2018-06-21T13:29:21-07:00
weight: 1
---

gg supports the fork-and-pull model of development [popularized by
GitHub][flow].

[flow]: https://guides.github.com/introduction/flow/

## Cloning

When setting up your working copy, you will first clone the repository as
normal, then add your fork as another remote. gg respects the
[`remote.pushDefault`][] Git configuration option on pushes, which reduces the
amount of typing you need to push your commits.

```shell
gg clone https://example.com/foo.git
cd foo
git remote add myfork https://example.com/myfork/foo.git
git config remote.pushDefault myfork
```

Replace the URLs and the name `myfork` above as needed.

[`remote.pushDefault`]: https://git-scm.com/docs/git-config#git-config-remotepushDefault

## Making Changes

Every change should be on a separate branch. `gg branch` will automatically
handle setting the branch's upstream. If you configure your working copy as
detailed in the Cloning section, `gg push` will automatically push your local
branch to your fork with the same branch name.

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

### Syncing Your Work with your Fork's Branch

If your fork's branch changes (for example, if a maintainer adds commits), then
first you need to download the new commits from your fork using `gg pull`.

```shell
gg pull -r CURRENT_BRANCH myfork
```

Replace `CURRENT_BRANCH` with the name of your current branch and `myfork` with
the name of the remote you added in the Cloning section.

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
