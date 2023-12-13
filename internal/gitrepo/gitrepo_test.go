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

package gitrepo

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"gg-scm.io/pkg/git/githash"
	"gg-scm.io/pkg/git/object"
	"github.com/google/go-cmp/cmp"
)

var _ interface {
	Repository
	Catter
} = Map(nil)

func TestMap(t *testing.T) {
	ctx := context.Background()
	var repo Map
	const content = "Hello, World!\n"
	wantHash, err := githash.ParseSHA1("8ab686eafeb1f44702738c8b0f24f2567c36da6d")
	if err != nil {
		t.Fatal(err)
	}

	wantPrefix := object.Prefix{
		Type: object.TypeBlob,
		Size: int64(len(content)),
	}
	gotHash, err := repo.WriteObject(ctx, wantPrefix, strings.NewReader(content))
	if gotHash != wantHash || err != nil {
		t.Errorf("WriteObject(...) = %v, %v; want %v, <nil>", gotHash, err, wantHash)
	}
	wantRepo := Map{
		wantHash: {Type: object.TypeBlob, Data: []byte(content)},
	}
	if diff := cmp.Diff(wantRepo, repo); diff != "" {
		t.Errorf("repo (-want +got):\n%s", diff)
	}

	gotPrefix, gotReader, err := repo.OpenObject(ctx, wantHash)
	if gotReader != nil {
		defer gotReader.Close()
	}
	if gotPrefix != wantPrefix || err != nil {
		t.Errorf("repo.OpenObject(ctx, %v) = %v, _, %v; want %v, _, <nil>",
			wantHash, gotPrefix, err, wantPrefix)
	}
	if gotReader != nil {
		gotContent, err := io.ReadAll(gotReader)
		if string(gotContent) != content || err != nil {
			t.Errorf("io.ReadAll(repo.OpenObject(ctx, %v)) = %q, %v; want %q, <nil>",
				wantHash, gotContent, err, content)
		}
	}
}

func TestCat(t *testing.T) {
	repo := make(Map)
	blobID := repo.Add(Object{
		Type: object.TypeBlob,
		Data: []byte("Hello, World!\n"),
	})
	treeData, err := (object.Tree{
		{Name: "hello.txt", Mode: object.ModePlain, ObjectID: blobID},
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	treeID := repo.Add(Object{
		Type: object.TypeTree,
		Data: treeData,
	})
	commitData, err := (&object.Commit{
		Tree:       treeID,
		Author:     "Ross Light <ross@zombiezen.com>",
		AuthorTime: time.Date(2023, time.December, 11, 14, 3, 0, 0, time.FixedZone("PST", -8*60*60)),
		Committer:  "Ross Light <ross@zombiezen.com>",
		CommitTime: time.Date(2023, time.December, 11, 14, 3, 0, 0, time.FixedZone("PST", -8*60*60)),
		Message:    "Initial import",
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	commitID := repo.Add(Object{
		Type: object.TypeCommit,
		Data: commitData,
	})
	badCommitData, err := (&object.Commit{
		Tree:       blobID,
		Author:     "Ross Light <ross@zombiezen.com>",
		AuthorTime: time.Date(2023, time.December, 11, 14, 3, 0, 0, time.FixedZone("PST", -8*60*60)),
		Committer:  "Ross Light <ross@zombiezen.com>",
		CommitTime: time.Date(2023, time.December, 11, 14, 3, 0, 0, time.FixedZone("PST", -8*60*60)),
		Message:    "Initial import",
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	badCommitID := repo.Add(Object{
		Type: object.TypeCommit,
		Data: badCommitData,
	})
	tagData, err := (&object.Tag{
		ObjectID:   commitID,
		ObjectType: object.TypeCommit,
		Tagger:     "Ross Light <ross@zombiezen.com>",
		Time:       time.Date(2023, time.December, 11, 14, 6, 0, 0, time.FixedZone("PST", -8*60*60)),
		Name:       "v1",
		Message:    "First version",
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	tagID := repo.Add(Object{
		Type: object.TypeTag,
		Data: tagData,
	})

	tests := []struct {
		name string
		id   githash.SHA1
		want map[object.Type][]byte
	}{
		{
			name: "Blob",
			id:   blobID,
			want: map[object.Type][]byte{
				object.TypeBlob: []byte("Hello, World!\n"),
			},
		},
		{
			name: "Tree",
			id:   treeID,
			want: map[object.Type][]byte{
				object.TypeTree: treeData,
			},
		},
		{
			name: "Commit",
			id:   commitID,
			want: map[object.Type][]byte{
				object.TypeTree:   treeData,
				object.TypeCommit: commitData,
			},
		},
		{
			name: "Tag",
			id:   tagID,
			want: map[object.Type][]byte{
				object.TypeTree:   treeData,
				object.TypeCommit: commitData,
				object.TypeTag:    tagData,
			},
		},
		{
			name: "BadCommit",
			id:   badCommitID,
			want: map[object.Type][]byte{
				object.TypeCommit: badCommitData,
			},
		},
		{
			name: "ZeroID",
			id:   githash.SHA1{},
			want: nil,
		},
	}
	knownTypes := [...]object.Type{
		object.TypeBlob, object.TypeTree, object.TypeCommit, object.TypeTag,
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			for _, tp := range knownTypes {
				buf := new(bytes.Buffer)
				want, wantsType := test.want[tp]
				err := Cat(ctx, repo, buf, tp, test.id)
				if !wantsType {
					if err == nil {
						t.Errorf("Cat(ctx, repo, buf, %q, %v) did not return error", tp, test.id)
					}
					continue
				}
				if err != nil {
					t.Errorf("Cat(ctx, repo, buf, %q, %v): %v", tp, test.id, err)
				}
				if diff := cmp.Diff(want, buf.Bytes()); diff != "" {
					t.Errorf("after Cat(ctx, repo, buf, %q, %v): buf.Bytes() (-want +got):\n%s",
						tp, test.id, diff)
				}
			}
		})
	}
}
