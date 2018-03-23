#!/bin/bash
mkdir -p "$1" || exit 1
cd "$1" || exit 1
curl -fsSL https://www.kernel.org/pub/software/scm/git/git-"$2".tar.xz | xz -d | tar xf - --strip-components=1 || exit 1
NO_GETTEXT=1 make -j2 git || exit 1
