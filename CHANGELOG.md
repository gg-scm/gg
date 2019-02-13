# Release Notes

## 0.7.0 (2019-02-13)

0.7 is a huge technical milestone for gg: most interactions with Git are now
going through a comprehensively tested Go library instead of constructing
command-line arguments ad-hoc. This means more input validation and less surface
area for bugs.

gg 0.7 drops support for Git 2.7.4: 2.11.0 is now the earliest supported version
of Git.

### Features

-  New command: [`identify`](https://gg-scm.io/cmd/identify/) to get the current
   commit hash. ([#94](https://github.com/zombiezen/gg/issues/94))
-  `commit` uses a Mercurial-like commit message template when the editor is
   invoked.
-  `pull` now pulls all refs from the remote instead of just one.
   ([#87](https://github.com/zombiezen/gg/issues/87))
-  The `update` command has been overhauled to be even closer to Mercurial
   semantics. `update` now uses "merge" behavior all the time
   ([#93](https://github.com/zombiezen/gg/issues/93)) and will fast-forward
   before switching the working copy to a new branch.
-  `update` now has a `--clean` flag
   ([#92](https://github.com/zombiezen/gg/issues/92))
-  `mail` now accepts comma-separated reviewers in `-R` flag.
   ([#83](https://github.com/zombiezen/gg/issues/83))
-  `gerrithook` now caches the hook script it downloads.
   ([#61](https://github.com/zombiezen/gg/issues/61))

### Bug Fixes

-  As mentioned in the introduction, most interactions with the Git subprocess
   go through a [high-level API](https://godoc.org/gg-scm.io/pkg/internal/git)
   now to reduce escaping issues. This should make gg more architecturally
   robust.
-  Previous versions of gg would never send `SIGTERM` correctly to its Git
   subprocess on receiving an interrupt, instead just waiting for the command
   to finish. This has been fixed.
-  `backout` now correctly attaches stdin to the editor.
   ([#85](https://github.com/zombiezen/gg/issues/85))
-  `push` now correctly consults the fetch URL when evaluating whether a push
   would result in a creation. ([#75](https://github.com/zombiezen/gg/issues/75))
-  gg now escapes pathspec characters on an as-needed basis, which should reduce
   most of the confusing error messages bubbled up from Git.
   ([#57](https://github.com/zombiezen/gg/issues/57))
-  `update` now uses more Mercurial-like logic for updating branches, correcting
   surprising behavior when pulling from fork branches.
   ([#80](https://github.com/zombiezen/gg/issues/80))
-  `update` now correctly merges the local state when given a branch name
   instead of exiting. ([#76](https://github.com/zombiezen/gg/issues/76))
-  `commit` no longer tries to commit untracked files when given a directory
   argument. ([#74](https://github.com/zombiezen/gg/issues/74))
-  `gerrithook` now respects the `core.hooksPath` configuration setting.
   ([#89](https://github.com/zombiezen/gg/issues/89))

### Known Issues

One of the known issues in 0.6 is still present in 0.7:

-   `revert` does not produce errors if you pass it unknown files.
    ([#58](https://github.com/zombiezen/gg/issues/58)). This may be confusing,
    but does not negatively affect your data.

## 0.6.1 (2018-08-22)

### Bug Fixes

-   `gg --version` no longer crashes ([#71](https://github.com/zombiezen/gg/issues/71))
-   `gg clone` does not try to clone to `/` ([#70](https://github.com/zombiezen/gg/issues/70))
-   `gg status` no longer crashes when encountering a renamed file on Git 2.11.
    ([#60](https://github.com/zombiezen/gg/issues/60))

## 0.6.0 (2018-07-17)

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

## 0.5.1 (2018-05-31)

### Bug Fixes

-   Running `commit` with no arguments no longer emits a misleading "fatal"
    error message. ([#43](https://github.com/zombiezen/gg/issues/43))

## 0.5.0 (2018-05-30)

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

## 0.4.2 (2018-05-02)

### Bug Fixes

-   `histedit` now works when passing a non-ref argument.
    ([#25](https://github.com/zombiezen/gg/issues/25))
-   `rebase -src` now behaves as documented when given a revision that is
    unrelated to HEAD. ([#27](https://github.com/zombiezen/gg/issues/27))

## 0.4.1 (2018-04-18)

### Bug Fixes

-   Instead of rebasing onto the upstream, `histedit` will keep the current
    branch at its fork point. This was always the intended behavior, but I
    didn't have a test, so I forgot to implement it.
    ([#20](https://github.com/zombiezen/gg/issues/20))

## 0.4.0 (2018-04-18)

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

## 0.3.0 (2018-04-06)

### Features

-   Add `evolve` command for syncing Gerrit changes.
    ([#14](https://github.com/zombiezen/gg/issues/14))
-   Add `gerrithook` command for installing or removing the
    [Gerrit Change ID](https://gerrit-review.googlesource.com/hooks/commit-msg)
    hook. ([#13](https://github.com/zombiezen/gg/issues/13))

### Bug Fixes

-   `rebase -dst` now always defaults to the upstream branch, whereas sometimes
    it would take from the source flags.

## 0.2.1 (2018-04-03)

### Bug Fixes

-   `histedit` flags no longer give incorrect usage errors. ([#11](https://github.com/zombiezen/gg/issues/11))

## 0.2.0 (2018-04-03)

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

## 0.1.3 (2018-03-23)

First public binary release.
