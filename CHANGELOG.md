# Release Notes

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
