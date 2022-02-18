# gg Release Notes

The format is based on [Keep a Changelog][], and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

[Keep a Changelog]: https://keepachangelog.com/en/1.0.0/
[Unreleased]: https://github.com/gg-scm/gg/compare/v1.1.0...HEAD

## [Unreleased][]

### Added

- gg can now be installed via [Nix](https://nixos.org/)!
  See https://gg-scm.io/install for instructions.
- `push` and `commit` have a new `-hooks=0` flag.

### Fixed

- `GIT_EDITOR` is now always invoked from the root of the working copy
  to match with the behavior of Git.
  ([#152](https://github.com/gg-scm/gg/issues/152))
- `revert` now prints an error message
  if used on a nonexistent file in a new repository.

## [1.1.0][] - 2020-12-13

Version 1.1 is the second stable release of gg and includes new commands,
improved `gg branch` output, simpler GitHub integration, and a Homebrew formula.

[1.1.0]: https://github.com/gg-scm/gg/releases/tag/v1.1.0

### Added

-  New `addremove` command that adds new files and removes missing ones.
   ([#95](https://github.com/gg-scm/gg/issues/95))
-  gg has a new command, `github-login`, which obtains a GitHub authorization
   token using a CLI-based OAuth flow. ([#122](https://github.com/gg-scm/gg/issues/122))
-  `branch` has a new `--sort` flag to control the sort order.
-  gg can now be installed via [Homebrew][]! See https://gg-scm.io/install for
   instructions.

[Homebrew]: https://brew.sh/

### Changed

-  `branch` shows the commit hash, author, and summary for each branch.
-  `branch` now sorts by descending commit date by default.
   ([#101](https://github.com/gg-scm/gg/issues/101))

### Fixed

-  `status` and `branch` now display color on Windows.
   ([#125](https://github.com/gg-scm/gg/issues/125))
-  Released binaries are smaller: they no longer contain debug information.
   ([#121](https://github.com/gg-scm/gg/issues/121))
-  `commit --amend` no longer exits with an error if the commit contains a
   rename. ([#129](https://github.com/gg-scm/gg/issues/129))
-  `rebase` displays a simpler error message if the `-dst` argument doesn't
   exist. ([#127](https://github.com/gg-scm/gg/issues/127))

## [1.0.3][] - 2020-09-07

### Added

-  gg now has pre-built Windows binaries!
   ([#48](https://github.com/gg-scm/gg/issues/48))

### Changed

-  The documentation has been moved to a [new repository](https://github.com/gg-scm/gg-scm.io).
   ([#119](https://github.com/gg-scm/gg/issues/119))

## [1.0.2][] - 2020-09-01

1.0.2 is an organizational release: gg moved to a [new GitHub organization](https://github.com/gg-scm),
released its internals as a [standalone Go library](https://github.com/gg-scm/gg-git),
and released Debian/Ubuntu packages!

[1.0.2]: https://github.com/gg-scm/gg/releases/tag/v1.0.2

### Added

-  gg now has an APT repository with Debian packages!
   ([#49](https://github.com/gg-scm/gg/issues/49))
   See https://gg-scm.io/ for installation instructions. Special thanks to my
   sponsors for covering hosting costs!

### Changed

-  `gg-scm.io/pkg/internal/git` is now available as `gg-scm.io/pkg/git`. To
   support this change, the main repository's import path has changed from
   `gg-scm.io/pkg` to `gg-scm.io/tool`.
-  gg has moved to a new organization on GitHub: https://github.com/gg-scm.
   Most URLs will redirect automatically, but please update any remotes or links
   pointing to `zombiezen/gg`.
-  CHANGELOG.md now uses the [Keep a Changelog][] format.

## [1.0.1][] - 2020-06-22

1.0.1 is a small bugfix release to 1.0.0.

[1.0.1]: https://github.com/gg-scm/gg/releases/tag/v1.0.1

### Fixed

- `gg push --new-branch` fails if not given a `-r` flag.

## [1.0.0][] - 2020-06-21

1.0 is the first stable release of gg, developed over years of daily use in a
variety of workflows. With tab completion and command semantics much closer to
Mercurial, the UX of gg has never been better.

gg 1.0 supports Git 2.20.1 and above.

[1.0.0]: https://github.com/gg-scm/gg/releases/tag/v1.0.0

### Added

- `gg pull` now pulls all branches and fast-forwards local ones if possible.
  When pulling from an unnamed remote repository, `gg pull` will place the
  branches into a `refs/ggpull/...` namespace.
  ([#108](https://github.com/gg-scm/gg/issues/108))
- `gg push` now pushes all branches unless limited with the `-r` flag.
  ([#100](https://github.com/gg-scm/gg/issues/100))
- Tab completion for bash and zsh. ([#18](https://github.com/gg-scm/gg/issues/18))
- `gg requestpull` accepts a new `-draft` flag.
  ([#104](https://github.com/gg-scm/gg/issues/104))
- `gg requestpull` will copy the repository's pull request template into the
  opened editor. ([#110](https://github.com/gg-scm/gg/issues/110))

### Changed

- The branch name created by `gg init` is now `main`.
  ([#115](https://github.com/gg-scm/gg/issues/115))
- `gg log` now logs all revisions by default. Use `gg log -r @` to get the old
  behavior. ([#86](https://github.com/gg-scm/gg/issues/86))
- `gg log` now always uses `git log --date-order` under the hood. As always, if
  you prefer tighter control over the log, use `git log` directly.
- `gg branch --delete` is now implemented in terms of `git update-ref` instead
  of `git branch --delete`.
- The `gg push --create` flag is now `gg push --new-branch` to match Mercurial.
- gg tests are now run by [GitHub Actions][] instead of Travis.
  [Shout out to Nat Friedman][Nat Friedman tweet] for the invite!
- When `gg requestpull` opens an editor, the file will have a `.md` extension.
  ([#113](https://github.com/gg-scm/gg/issues/113))
- `gg histedit` will now set the `--autosquash` option when running `git rebase`.
  ([#114](https://github.com/gg-scm/gg/issues/114))

[GitHub Actions]: https://github.com/features/actions
[Nat Friedman tweet]: https://twitter.com/natfriedman/status/1162822908411965441

### Fixed

- `gg update` no longer errors if local branch is ahead of remote branch.
  ([#103](https://github.com/gg-scm/gg/issues/103))
- `gg rebase --continue` no longer adds untracked files
  ([#107](https://github.com/gg-scm/gg/issues/107))
- `gg gerrithook` will now work properly when in run in a subdirectory of the
  Git repository. ([#105](https://github.com/gg-scm/gg/issues/105))
- `gg commit -amend` now works on the repository's first commit.
  ([#106](https://github.com/gg-scm/gg/issues/106))
- `gg revert` will show an error message if it is given a file that doesn't
  exist. ([#58](https://github.com/gg-scm/gg/issues/58))
- `gg branch` will no longer fail on an empty repository.

## [0.7.1][] - 2019-02-13

The release scripts for 0.7.0 failed, so 0.7.1 is the first actual release of
0.7.

[0.7.1]: https://github.com/gg-scm/gg/releases/tag/v0.7.1

## [0.7.0][] - 2019-02-13

0.7 is a huge technical milestone for gg: most interactions with Git are now
going through a comprehensively tested Go library instead of constructing
command-line arguments ad-hoc. This means more input validation and less surface
area for bugs.

gg 0.7 drops support for Git 2.7.4: 2.11.0 is now the earliest supported version
of Git.

[0.7.0]: https://github.com/gg-scm/gg/releases/tag/v0.7.0

### Added

-  New command: [`identify`](https://gg-scm.io/cmd/identify/) to get the current
   commit hash. ([#94](https://github.com/gg-scm/gg/issues/94))
-  `commit` uses a Mercurial-like commit message template when the editor is
   invoked.
-  `pull` now pulls all refs from the remote instead of just one.
   ([#87](https://github.com/gg-scm/gg/issues/87))
-  The `update` command has been overhauled to be even closer to Mercurial
   semantics. `update` now uses "merge" behavior all the time
   ([#93](https://github.com/gg-scm/gg/issues/93)) and will fast-forward
   before switching the working copy to a new branch.
-  `update` now has a `--clean` flag
   ([#92](https://github.com/gg-scm/gg/issues/92))
-  `mail` now accepts comma-separated reviewers in `-R` flag.
   ([#83](https://github.com/gg-scm/gg/issues/83))
-  `gerrithook` now caches the hook script it downloads.
   ([#61](https://github.com/gg-scm/gg/issues/61))

### Fixed

-  As mentioned in the introduction, most interactions with the Git subprocess
   go through a [high-level API](https://pkg.go.dev/gg-scm.io/pkg/git)
   now to reduce escaping issues. This should make gg more architecturally
   robust.
-  Previous versions of gg would never send `SIGTERM` correctly to its Git
   subprocess on receiving an interrupt, instead just waiting for the command
   to finish. This has been fixed.
-  `backout` now correctly attaches stdin to the editor.
   ([#85](https://github.com/gg-scm/gg/issues/85))
-  `push` now correctly consults the fetch URL when evaluating whether a push
   would result in a creation. ([#75](https://github.com/gg-scm/gg/issues/75))
-  gg now escapes pathspec characters on an as-needed basis, which should reduce
   most of the confusing error messages bubbled up from Git.
   ([#57](https://github.com/gg-scm/gg/issues/57))
-  `update` now uses more Mercurial-like logic for updating branches, correcting
   surprising behavior when pulling from fork branches.
   ([#80](https://github.com/gg-scm/gg/issues/80))
-  `update` now correctly merges the local state when given a branch name
   instead of exiting. ([#76](https://github.com/gg-scm/gg/issues/76))
-  `commit` no longer tries to commit untracked files when given a directory
   argument. ([#74](https://github.com/gg-scm/gg/issues/74))
-  `gerrithook` now respects the `core.hooksPath` configuration setting.
   ([#89](https://github.com/gg-scm/gg/issues/89))

### Known Issues

One of the known issues in 0.6 is still present in 0.7:

-   `revert` does not produce errors if you pass it unknown files.
    ([#58](https://github.com/gg-scm/gg/issues/58)). This may be confusing,
    but does not negatively affect your data.

## [0.6.1][] - 2018-08-22

[0.6.1]: https://github.com/gg-scm/gg/releases/tag/v0.6.1

### Fixed

-   `gg --version` no longer crashes ([#71](https://github.com/gg-scm/gg/issues/71))
-   `gg clone` does not try to clone to `/` ([#70](https://github.com/gg-scm/gg/issues/70))
-   `gg status` no longer crashes when encountering a renamed file on Git 2.11.
    ([#60](https://github.com/gg-scm/gg/issues/60))

## [0.6.0][] - 2018-07-17

[0.6.1]: https://github.com/gg-scm/gg/releases/tag/v0.6.1

### Added

-   New website with docs! Take a look at [gg-scm.io](https://gg-scm.io/).
    The site includes workflow guides and the command reference.
    ([#23](https://github.com/gg-scm/gg/issues/23),
    [#40](https://github.com/gg-scm/gg/issues/40))
-   The new [`requestpull` command](https://gg-scm.io/cmd/requestpull/) creates
    GitHub pull requests from the command line.
    ([#52](https://github.com/gg-scm/gg/issues/52))
-   `revert` now creates backup files when reverting modified files. See the
    [`revert` docs](https://gg-scm.io/cmd/revert/) for more details.
    ([#39](https://github.com/gg-scm/gg/issues/39))
-   The new [`backout` command](https://gg-scm.io/cmd/backout/) creates commits
    that "undo" the effect of previous commits.
    ([#46](https://github.com/gg-scm/gg/issues/46))
-   The new [`cat` command](https://gg-scm.io/cmd/cat/) allows viewing files
    from old commits. ([#45](https://github.com/gg-scm/gg/issues/45))
-   Ignored files can be tracked by naming them explicitly using `add`.
    ([#51](https://github.com/gg-scm/gg/issues/51))

### Fixed

-   `status` no longer crashes on newer versions of Git that detect renames in
    the working copy. ([#44](https://github.com/gg-scm/gg/issues/44))
-   `revert` now correctly operates on added files
    ([#54](https://github.com/gg-scm/gg/issues/54),
    [#55](https://github.com/gg-scm/gg/issues/55))
-   `add` is now more robust when passed a directory.
    ([#35](https://github.com/gg-scm/gg/issues/35))
-   Git subprocesses are now sent `SIGTERM` if gg is sent any interrupt or
    termination signals. ([#64](https://github.com/gg-scm/gg/issues/64))

### Known Issues

There are a few known issues in 0.6:

-   `revert` does not produce errors if you pass it unknown files.
    ([#58](https://github.com/gg-scm/gg/issues/58)). This may be confusing,
    but does not negatively affect your data.
-   Git 2.11 (the default version used on Debian) emits malformed output on
    working copy renames which causes gg to crash.
    ([#60](https://github.com/gg-scm/gg/issues/60)) The workaround here is to
    install a newer version of Git.

## [0.5.1][] - 2018-05-31

[0.5.1]: https://github.com/gg-scm/gg/releases/tag/v0.5.1

### Fixed

-   Running `commit` with no arguments no longer emits a misleading "fatal"
    error message. ([#43](https://github.com/gg-scm/gg/issues/43))

## [0.5.0][] - 2018-05-30

[0.5.0]: https://github.com/gg-scm/gg/releases/tag/v0.5.0

### Added

-   `diff` now has more flags for controlling comparisons: `-U`,
    `--ignore-space-change`, `--ignore-blank-lines`, `--ignore-all-space`,
    `--ignore-space-at-eol`, `-M`, `-C`, and `--copies-unmodified`. See `gg
    help diff` for more details.
    ([#26](https://github.com/gg-scm/gg/issues/26))
-   `log` now has more flags for controlling output:
    `--reverse`, `--stat`, and `--follow-first`.
    ([#22](https://github.com/gg-scm/gg/issues/22) and
    [#41](https://github.com/gg-scm/gg/issues/41))
-   `push` and `mail` now infer the destination ref for commits that are only
    present in one branch. For example, a command like `gg push -r main~`
    will push to `main` on the destination.
    ([#31](https://github.com/gg-scm/gg/issues/31))
-   `rebase --continue` and `histedit --continue` will now
    automatically amend the current commit if changes were made.
    ([#21](https://github.com/gg-scm/gg/issues/21))
-   `rm` has a `-r` flag
    ([#24](https://github.com/gg-scm/gg/issues/24))

### Fixed

-   Fix crash when running `diff` before first commit
    ([#30](https://github.com/gg-scm/gg/issues/30))
-   When `push` checks whether a ref exists for `-create`, it will use the
    push URL instead of the fetch URL.
    ([#28](https://github.com/gg-scm/gg/issues/28))
-   Running `add` on the working directory root now works.
    ([#29](https://github.com/gg-scm/gg/issues/29))
-   `rebase` and `histedit` no longer use `--fork-point`, since it can
    cause unexpected, hard-to-debug results.
-   `commit` now works properly when finishing a merge.
    ([#38](https://github.com/gg-scm/gg/issues/38))
-   `merge` no longer creates a commit.
    ([#42](https://github.com/gg-scm/gg/issues/42))

## [0.4.2][] - 2018-05-02

[0.4.2]: https://github.com/gg-scm/gg/releases/tag/v0.4.2

### Fixed

-   `histedit` now works when passing a non-ref argument.
    ([#25](https://github.com/gg-scm/gg/issues/25))
-   `rebase -src` now behaves as documented when given a revision that is
    unrelated to HEAD. ([#27](https://github.com/gg-scm/gg/issues/27))

## [0.4.1][] - 2018-04-18

[0.4.1]: https://github.com/gg-scm/gg/releases/tag/v0.4.1

### Fixed

-   Instead of rebasing onto the upstream, `histedit` will keep the current
    branch at its fork point. This was always the intended behavior, but I
    didn't have a test, so I forgot to implement it.
    ([#20](https://github.com/gg-scm/gg/issues/20))

## [0.4.0][] - 2018-04-18

[0.4.0]: https://github.com/gg-scm/gg/releases/tag/v0.4.0

### Added

-   Add `upstream` command for querying and setting upstream branches.
    ([#15](https://github.com/gg-scm/gg/issues/15))
-   `add` can now be used to mark unmerged files as resolved.
    ([#12](https://github.com/gg-scm/gg/issues/12))
-   gg commands that read configuration settings from git now do so in a single
    batch instead of invoking Git multiple times. This potentially improves
    performance of certain commands like status.
-   `histedit` now has an `-exec` flag, mirroring `git rebase --exec`.
-   `mail` now has flags to control sending of notifications.
-   `pull` now fetches all tags from a remote by default. Use `-tags=0` to
    disable this behavior. ([#17](https://github.com/gg-scm/gg/issues/17))

## [0.3.0][] - 2018-04-06

[0.3.0]: https://github.com/gg-scm/gg/releases/tag/v0.3.0

### Added

-   Add `evolve` command for syncing Gerrit changes.
    ([#14](https://github.com/gg-scm/gg/issues/14))
-   Add `gerrithook` command for installing or removing the
    [Gerrit Change ID](https://gerrit-review.googlesource.com/hooks/commit-msg)
    hook. ([#13](https://github.com/gg-scm/gg/issues/13))

### Fixed

-   `rebase -dst` now always defaults to the upstream branch, whereas sometimes
    it would take from the source flags.

## [0.2.1][] - 2018-04-03

[0.2.1]: https://github.com/gg-scm/gg/releases/tag/v0.2.1

### Added

-   `histedit` flags no longer give incorrect usage errors. ([#11](https://github.com/gg-scm/gg/issues/11))

## [0.2.0][] - 2018-04-03

[0.2.0]: https://github.com/gg-scm/gg/releases/tag/v0.2.0

### Added

-   `push` has a new `-f` flag that uses `git push --force-with-lease`.
    ([#9](https://github.com/gg-scm/gg/issues/9))
-   `mail` now warns if there are uncommitted changes.
    ([#8](https://github.com/gg-scm/gg/issues/8))
-   `status` will use colorized output when appropriate. It can be controlled
    using the `color.ggstatus` configuration setting.
    ([#1](https://github.com/gg-scm/gg/issues/1))
-   Added the `rebase` and `histedit` commands.
    ([#5](https://github.com/gg-scm/gg/issues/5))
-   Added the `merge` command.
    ([#6](https://github.com/gg-scm/gg/issues/6))

### Fixed

-   `commit -amend` with no changes now opens the editor instead of failing.
    ([#7](https://github.com/gg-scm/gg/issues/7))
-   Running `commit` in a subdirectory of the working copy will no longer fail
    when committing a removed file. ([#10](https://github.com/gg-scm/gg/issues/10))

## [0.1.3][] - 2018-03-23

First public binary release.

[0.1.3]: https://github.com/gg-scm/gg/releases/tag/v0.1.3
