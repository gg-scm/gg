# Release Notes

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
