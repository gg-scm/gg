# release/ directory

This directory holds release automation scripts for gg.

## Release Checklist

1. Push last code commit up to CI
1. Prepare Nixpkgs PR
1. New CHANGELOG section
1. Update [default.nix](../default.nix) version number
1. Update [misc/gg.1.md](../misc/gg.1.md) version number
1. `gg commit && git tag -a vX.Y.Z`
1. `gg push`
1. [Publish new release on GitHub](https://github.com/gg-scm/gg/releases/new)
1. [Update Homebrew formula](https://github.com/gg-scm/homebrew-gg/edit/main/Formula/gg.rb)
1. `go run ./misc/extractdocs -touch=false ../docs/content/commands/`
1. [Update docs version](https://github.com/gg-scm/gg-scm.io/edit/main/config.toml)
