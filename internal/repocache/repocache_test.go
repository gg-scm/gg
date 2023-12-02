// Copyright 2023 The gg Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//		 https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package repocache

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gg-scm.io/pkg/git"
	"gg-scm.io/pkg/git/object"
	"gg-scm.io/pkg/git/packfile/client"
	"gg-scm.io/tool/internal/filesystem"
	"github.com/google/go-cmp/cmp"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func TestOpen(t *testing.T) {
	t.Run("Clean", func(t *testing.T) {
		ctx := context.Background()
		cache, err := Open(ctx, filepath.Join(t.TempDir(), "foo.db"))
		if err != nil {
			t.Fatal(err)
		}
		if err := cache.Close(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("DifferentVersion", func(t *testing.T) {
		ctx := context.Background()
		dbPath := filepath.Join(t.TempDir(), "foo.db")
		conn, err := sqlite.OpenConn(dbPath, sqlite.OpenCreate|sqlite.OpenReadWrite)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		err = sqlitex.ExecuteTransient(conn, fmt.Sprintf("PRAGMA application_id = %d;", appID), nil)
		if err != nil {
			t.Fatal(err)
		}
		err = sqlitex.ExecuteTransient(conn, fmt.Sprintf("PRAGMA user_version = %d;", currentUserVersion+1), nil)
		if err != nil {
			t.Fatal(err)
		}
		err = sqlitex.ExecuteTransient(conn, "CREATE TABLE testjunktable (foo);", nil)
		if err != nil {
			t.Fatal(err)
		}

		cache, err := Open(ctx, dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := cache.Close(); err != nil {
			t.Fatal(err)
		}

		err = sqlitex.ExecuteTransient(conn, "VALUES (EXISTS(SELECT 1 FROM sqlite_schema WHERE name = 'testjunktable'));", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				if stmt.ColumnBool(0) {
					t.Error("testjunktable exists")
				}
				return nil
			},
		})
		if err != nil {
			t.Error(err)
		}

		err = sqlitex.ExecuteTransient(conn, "PRAGMA user_version;", &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				if got := stmt.ColumnInt(0); got != currentUserVersion {
					t.Errorf("user_version = %d; want %d", got, currentUserVersion)
				}
				return nil
			},
		})
		if err != nil {
			t.Error(err)
		}
	})
}

func TestCopyFrom(t *testing.T) {
	const (
		fileName     = "foo.txt"
		fileContents = "Hello, World!\n"

		commitMessage             = "Initial import"
		commitAuthor  object.User = "Ross Light <ross@zombiezen.com>"
	)
	commitTime := time.Date(2023, time.December, 2, 17, 30, 0, 0, time.UTC)

	ctx := context.Background()
	gitDir := filesystem.Dir(t.TempDir())
	g, err := git.New(git.Options{Dir: gitDir.String()})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Init(ctx, "."); err != nil {
		t.Fatal(err)
	}
	err = gitDir.Apply(filesystem.Write(fileName, fileContents))
	if err != nil {
		t.Fatal(err)
	}
	err = g.Add(ctx, []git.Pathspec{git.LiteralPath(fileName)}, git.AddOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = g.Commit(ctx, commitMessage, git.CommitOptions{
		Author:     commitAuthor,
		AuthorTime: commitTime,
		Committer:  commitAuthor,
		CommitTime: commitTime,
	})
	if err != nil {
		t.Fatal(err)
	}
	fileObjectName, err := object.BlobSum(strings.NewReader(fileContents), int64(len(fileContents)))
	if err != nil {
		t.Fatal(err)
	}
	treeObjectName := object.Tree{{
		Name:     fileName,
		Mode:     object.ModePlain,
		ObjectID: fileObjectName,
	}}.SHA1()
	commitObject := &object.Commit{
		Tree:       treeObjectName,
		Author:     commitAuthor,
		AuthorTime: commitTime,
		Committer:  commitAuthor,
		CommitTime: commitTime,
		Message:    commitMessage,
	}
	want, err := commitObject.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	commitObjectName := commitObject.SHA1()
	headRev, err := g.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if headRev.Commit != commitObjectName {
		t.Fatalf("%s = %v; want %v", git.Head, headRev.Commit, commitObjectName)
	}

	cache, err := Open(ctx, filepath.Join(t.TempDir(), "foo.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			t.Error(err)
		}
	}()
	gitClient, err := client.NewRemote(client.URLFromPath(gitDir.String()), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.CopyFrom(ctx, gitClient); err != nil {
		t.Error("CopyFrom:", err)
	}

	got := new(bytes.Buffer)
	gotType, err := cache.Cat(ctx, got, commitObjectName)
	if err != nil {
		t.Fatal("Cat:", err)
	}
	if wantType := object.TypeCommit; gotType != wantType {
		t.Errorf("type = %q; want %q", gotType, wantType)
	}
	if diff := cmp.Diff(want, got.Bytes()); diff != "" {
		t.Errorf("content (-want +got):\n%s", diff)
	}
}
