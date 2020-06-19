// Copyright 2019 The gg Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
	"gg-scm.io/pkg/internal/git"
)

func TestIdentify(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("Empty", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.initEmptyRepo(ctx, "."); err != nil {
			t.Fatal(err)
		}

		_, err = env.gg(ctx, env.root.String(), "identify")
		if err == nil {
			t.Error("gg did not return error")
		} else if isUsage(err) {
			t.Errorf("gg returned usage error: %v", err)
		}
	})
	t.Run("NoArgsClean", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		head, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}

		got, err := env.gg(ctx, env.root.String(), "identify")
		if err != nil {
			t.Error(err)
		}
		if want := head.Commit.String() + " main\n"; string(got) != want {
			t.Errorf("identify output = %q; want %q", got, want)
		}
	})
	t.Run("UntrackedFile", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		head, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("file.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}

		got, err := env.gg(ctx, env.root.String(), "identify")
		if err != nil {
			t.Error(err)
		}
		if want := head.Commit.String() + " main\n"; string(got) != want {
			t.Errorf("identify output = %q; want %q", got, want)
		}
	})
	t.Run("AddedFile", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		head, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("file.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.git.Add(ctx, []git.Pathspec{"file.txt"}, git.AddOptions{}); err != nil {
			t.Fatal(err)
		}

		got, err := env.gg(ctx, env.root.String(), "identify")
		if err != nil {
			t.Error(err)
		}
		if want := head.Commit.String() + "+ main\n"; string(got) != want {
			t.Errorf("identify output = %q; want %q", got, want)
		}
	})
	t.Run("RevFlag", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		prev, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("file.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.git.Add(ctx, []git.Pathspec{"file.txt"}, git.AddOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.git.Commit(ctx, "top commit\n", git.CommitOptions{}); err != nil {
			t.Fatal(err)
		}

		got, err := env.gg(ctx, env.root.String(), "identify", "-r", "HEAD~")
		if err != nil {
			t.Error(err)
		}
		if want := prev.Commit.String() + "\n"; string(got) != want {
			t.Errorf("identify output = %q; want %q", got, want)
		}
	})
	t.Run("RevFlagWithLocalMods", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		prev, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("file.txt", "first\n")); err != nil {
			t.Fatal(err)
		}
		if err := env.git.Add(ctx, []git.Pathspec{"file.txt"}, git.AddOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.git.Commit(ctx, "top commit\n", git.CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("file.txt", "something new\n")); err != nil {
			t.Fatal(err)
		}

		got, err := env.gg(ctx, env.root.String(), "identify", "-r", "HEAD~")
		if err != nil {
			t.Error(err)
		}
		if want := prev.Commit.String() + "\n"; string(got) != want {
			t.Errorf("identify output = %q; want %q", got, want)
		}
	})
	t.Run("Tag", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		head, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.git.Run(ctx, "tag", "foo"); err != nil {
			t.Fatal(err)
		}

		got, err := env.gg(ctx, env.root.String(), "identify")
		if err != nil {
			t.Error(err)
		}
		if want := head.Commit.String() + " main foo\n"; string(got) != want {
			t.Errorf("identify output = %q; want %q", got, want)
		}
	})
	t.Run("AnnotatedTag", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		head, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.git.Run(ctx, "tag", "-a", "-m", "the foo tag", "foo"); err != nil {
			t.Fatal(err)
		}

		got, err := env.gg(ctx, env.root.String(), "identify")
		if err != nil {
			t.Error(err)
		}
		if want := head.Commit.String() + " main foo\n"; string(got) != want {
			t.Errorf("identify output = %q; want %q", got, want)
		}
	})
	t.Run("AnnotatedTagArg", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		head, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.git.Run(ctx, "tag", "-a", "-m", "the foo tag", "foo"); err != nil {
			t.Fatal(err)
		}

		got, err := env.gg(ctx, env.root.String(), "identify", "-r", "foo")
		if err != nil {
			t.Error(err)
		}
		if want := head.Commit.String() + " main foo\n"; string(got) != want {
			t.Errorf("identify output = %q; want %q", got, want)
		}
	})
}
