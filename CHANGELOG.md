# Release Notes

## 0.6.0

### Features

-   New website with docs! Take a look at [gg-scm.io](https://gg-scm.io/).
    The site includes workflow guides and the command reference.
    ([#23](https://github.com/zombiezen/gg/issues/23),
    [#40](https://github.com/zombiezen/gg/issues/40))
-   The new [`requestpull` command](https://gg-scm.io/cmd/requestpull/) creates
    GitHub pull requests from the command line.
    ([#52](https://github.com/zombiezen/gg/issues/52))
-   `revert` now creates backup files when reverting modified files. See the
    [`revert` docs](https://gg-scm.io/cmd/revert/) for more details.
    ([#39](https://github.com/zombiezen/gg/issues/39))
-   The new [`backout` command](https://gg-scm.io/cmd/backout/) creates commits
    that "undo" the effect of previous commits.
    ([#46](https://github.com/zombiezen/gg/issues/46))
-   The new [`cat` command](https://gg-scm.io/cmd/cat/) allows viewing files
    from old commits. ([#45](https://github.com/zombiezen/gg/issues/45))
-   Ignored files can be tracked by naming them explicitly using `add`.
    ([#51](https://github.com/zombiezen/gg/issues/51))

### Bug Fixes

-   `status` no longer crashes on newer versions of Git that detect renames in
    the working copy. ([#44](https://github.com/zombiezen/gg/issues/44))
-   `revert` now correctly operates on added files
    ([#54](https://github.com/zombiezen/gg/issues/54),
    [#55](https://github.com/zombiezen/gg/issues/55))
-   `add` is now more robust when passed a directory.
    ([#35](https://github.com/zombiezen/gg/issues/35))
-   Git subprocesses are now sent `SIGTERM` if gg is sent any interrupt or
    termination signals. ([#64](https://github.com/zombiezen/gg/issues/64))

### Known Issues

There are a few known issues in 0.6:

-   `revert` does not produce errors if you pass it unknown files.
    ([#58](https://github.com/zombiezen/gg/issues/58)). This may be confusing,
    but does not negatively affect your data.
-   Git 2.11 (the default version used on Debian) emits malformed output on
    working copy renames which causes gg to crash.
    ([#60](https://github.com/zombiezen/gg/issues/60)) The workaround here is to
    install a newer version of Git.

## 0.5.1

### Bug Fixes

-   Running `commit` with no arguments no longer emits a misleading "fatal"
    error message. ([#43](https://github.com/zombiezen/gg/issues/43))

## 0.5.0

### Features

-   `diff` now has more flags for controlling comparisons: `-U`,
    `--ignore-space-change`, `--ignore-blank-lines`, `--ignore-all-space`,
    `--ignore-space-at-eol`, `-M`, `-C`, and `--copies-unmodified`. See `gg
    help diff` for more details.
    ([#26](https://github.com/zombiezen/gg/issues/26))
-   `log` now has more flags for controlling output:
    `--reverse`, `--stat`, and `--follow-first`.
    ([#22](https://github.com/zombiezen/gg/issues/22) and
    [#41](https://github.com/zombiezen/gg/issues/41))
-   `push` and `mail` now infer the destination ref for commits that are only
    present in one branch. For example, a command like `gg push -r master~`
    will push to `master` on the destination.
    ([#31](https://github.com/zombiezen/gg/issues/31))
-   `rebase --continue` and `histedit --continue` will now
    automatically amend the current commit if changes were made.
    ([#21](https://github.com/zombiezen/gg/issues/21))
-   `rm` has a `-r` flag
    ([#24](https://github.com/zombiezen/gg/issues/24))

### Bug Fixes

-   Fix crash when running `diff` before first commit
    ([#30](https://github.com/zombiezen/gg/issues/30))
-   When `push` checks whether a ref exists for `-create`, it will use the
    push URL instead of the fetch URL.
    ([#28](https://github.com/zombiezen/gg/issues/28))
-   Running `add` on the working directory root now works.
    ([#29](https://github.com/zombiezen/gg/issues/29))
-   `rebase` and `histedit` no longer use `--fork-point`, since it can
    cause unexpected, hard-to-debug results.
-   `commit` now works properly when finishing a merge.
    ([#38](https://github.com/zombiezen/gg/issues/38))
-   `merge` no longer creates a commit.
    ([#42](https://github.com/zombiezen/gg/issues/42))

## 0.4.2

### Bug Fixes

-   `histedit` now works when passing a non-ref argument.
    ([#25](https://github.com/zombiezen/gg/issues/25))
-   `rebase -src` now behaves as documented when given a revision that is
    unrelated to HEAD. ([#27](https://github.com/zombiezen/gg/issues/27))

## 0.4.1

### Bug Fixes

-   Instead of rebasing onto the upstream, `histedit` will keep the current
    branch at its fork point. This was always the intended behavior, but I
    didn't have a test, so I forgot to implement it.
    ([#20](https://github.com/zombiezen/gg/issues/20))

## 0.4.0

### Features

-   Add `upstream` command for querying and setting upstream branches.
    ([#15](https://github.com/zombiezen/gg/issues/15))
-   `add` can now be used to mark unmerged files as resolved.
    ([#12](https://github.com/zombiezen/gg/issues/12))
-   gg commands that read configuration settings from git now do so in a single
    batch instead of invoking Git multiple times. This potentially improves
    performance of certain commands like status.
-   `histedit` now has an `-exec` flag, mirroring `git rebase --exec`.
-   `mail` now has flags to control sending of notifications.
-   `pull` now fetches all tags from a remote by default. Use `-tags=0` to
    disable this behavior. ([#17](https://github.com/zombiezen/gg/issues/17))

## 0.3.0

### Features

-   Add `evolve` command for syncing Gerrit changes.
    ([#14](https://github.com/zombiezen/gg/issues/14))
-   Add `gerrithook` command for installing or removing the
    [Gerrit Change ID](https://gerrit-review.googlesource.com/hooks/commit-msg)
    hook. ([#13](https://github.com/zombiezen/gg/issues/13))

### Bug Fixes

-   `rebase -dst` now always defaults to the upstream branch, whereas sometimes
    it would take from the source flags.

## 0.2.1

### Bug Fixes

-   `histedit` flags no longer give incorrect usage errors. ([#11](https://github.com/zombiezen/gg/issues/11))

## 0.2.0

### Features

-   `push` has a new `-f` flag that uses `git push --force-with-lease`.
    ([#9](https://github.com/zombiezen/gg/issues/9))
-   `mail` now warns if there are uncommitted changes.
    ([#8](https://github.com/zombiezen/gg/issues/8))
-   `status` will use colorized output when appropriate. It can be controlled
    using the `color.ggstatus` configuration setting.
    ([#1](https://github.com/zombiezen/gg/issues/1))
-   Added the `rebase` and `histedit` commands.
    ([#5](https://github.com/zombiezen/gg/issues/5))
-   Added the `merge` command.
    ([#6](https://github.com/zombiezen/gg/issues/6))

### Bug Fixes

-   `commit -amend` with no changes now opens the editor instead of failing.
    ([#7](https://github.com/zombiezen/gg/issues/7))
-   Running `commit` in a subdirectory of the working copy will no longer fail
    when committing a removed file. ([#10](https://github.com/zombiezen/gg/issues/10))

## 0.1.3

First public binary release.
