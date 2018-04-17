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

-   Push and pull only operate on one ref at a time.  The Git CLI does not
    provide enough control over multi-ref pulls and pushes without additional
    configuration variables.

## Testing

-   Integration testing gives much more confidence than unit testing, as most of
    gg is defined by its end effect on a Git repository.  Prefer integration
    tests in this project, but if there are small spot checks that make more
    sense as unit tests (testing a git output parser, for instance), these are
    fine, as long as the flow it's used in is covered by an integration test.
    This way, we can catch git version differences (of which there are many).
