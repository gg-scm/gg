# How to Contribute

We'd love to accept your patches and contributions to this project. There are
just a few small guidelines you need to follow.

## What to Contribute

**Issues:** Report bugs or request features using [GitHub issues][issues].

**Documentation:** You can send documentation changes via pull requests to the
[website repository][].

**Bug fixes:** You can send bug fixes via pull requests.

**Tests:** gg aims to be a stable part of a developer workflow, so it's vitally
important that it is robust to a wide variety of failures.  Even if nothing is
broken, adding new tests is always welcome.

**Features:** If you want to add a feature, please discuss it in on the issue
tracker first. Also, take a look at [DESIGN.md][], which outlines the scope of
features and the ideas behind the project.

[DESIGN.md]: https://github.com/gg-scm/gg/blob/main/DESIGN.md
[issues]: https://github.com/gg-scm/gg/issues
[website repository]: https://github.com/gg-scm/gg-scm.io

## Building from Source

You must have Go 1.16 or later to build gg.

```
# From a release tarball or a local clone:
release/build.bash ~/bin/gg

# Or using the go tool:
go install ./cmd/gg
```

## Code reviews

All submissions require review. We use GitHub pull requests for this purpose.
Consult [GitHub Help](https://help.github.com/articles/about-pull-requests/) for
more information on using pull requests.
