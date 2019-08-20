# gg Design Principles

-   **gg should be good enough to replace the git CLI for common workflows, but
    gg should not replace git.** Asking the user to duck down into git for some
    arcana is perfectly acceptable if the alternative is adding complexity to
    gg.
-   **Every gg command should map cleanly to a sequence of git commands.**  It
    is fine for gg's implementation to interact with the .git directory directly
    for efficiency, but only if a set of git CLI invocations would produce
    equivalent results.
-   **Focus on pull request and Gerrit change workflows.**
-   **Strive for Mercurial's command set, but don't be beholden to it.**  For
    example, gg uses git's revision parsing logic instead of trying to replicate
    Mercurial's.  Branches act like Mercurial bookmarks rather than Mercurial's
    branches, since Git doesn't have an equivalent concept.  Simplicity is
    preferred over exact compatibility.

## Specific decisions

-   gg commands should act as if there is no index, but make an effort to not
    clobber the index. This allows more advanced git workflows, while keeping
    the gg workflow simple. For example, `gg commit` works by passing files
    directly to `git commit`, so in case `git commit` aborts, then the
    index is not modified.
-   Git does not have a strong concept of "descendants", whereas many Mercurial
    commands are convenient because they do have a concept of what changesets
    descend from a given changeset. The canonical example is `hg rebase -src`,
    which takes every descending changeset and moves it on top of another
    changeset. Since this significantly improves the developer experience, I
    have opted to define descendants of commit X in Git as *any commit reachable
    by one or more references under `refs/heads/` that contains X*.
-   When pushing or pulling changes, gg maps the local branch names directly to
    the remote branch names. The upstream branch should only be consulted for
    merge-related decisions. This matches how pull request flows typically work
    anyway, and allows `gg pull` to operate the same on a random URL as a named
    remote. For Gerrit workflows (the primary case where I would choose a
    differing push destination), `gg mail` works fine.

## Testing

-   Integration testing gives much more confidence than unit testing, as most of
    gg is defined by its end effect on a Git repository.  Prefer integration
    tests in this project, but if there are small spot checks that make more
    sense as unit tests (testing a git output parser, for instance), these are
    fine, as long as the flow it's used in is covered by an integration test.
    This way, we can catch git version differences (of which there are many).

-   As of gg 0.7, the `internal/git` package is now the preferred way of
    interacting with Git, even in tests. `internal/git` is tested using raw
    Git commands to ensure that the command-line invocations are correct, and
    then gg integration tests use `internal/git` so that they are using
    well-tested and structured Git invocations.

-   gg is tested on the version of Git in the latest Ubuntu LTS release,
    the version in the latest stable Debian release, and the latest release
    version.
