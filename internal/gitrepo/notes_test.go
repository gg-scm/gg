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
	"testing"

	"gg-scm.io/pkg/git/githash"
	"gg-scm.io/pkg/git/object"
)

func TestNotesForCommit(t *testing.T) {
	commit1, err := githash.ParseSHA1("85be80f2ff7482f53f6581be8683036d7a59c0e7")
	if err != nil {
		t.Fatal(err)
	}
	commit2, err := githash.ParseSHA1("af3ab0b8ddca5513f86812d820802202514ed986")
	if err != nil {
		t.Fatal(err)
	}
	commit3, err := githash.ParseSHA1("3134253d1adfa21be31624f6d48243e18651db68")
	if err != nil {
		t.Fatal(err)
	}

	repo := make(Map)
	commit2Notes := []byte("Commit #2\n")
	commit2NotesBlob := repo.Add(Object{
		Type: object.TypeBlob,
		Data: commit2Notes,
	})
	subtreeData, err := (object.Tree{
		{
			Name:     "3ab0b8ddca5513f86812d820802202514ed986",
			Mode:     object.ModePlain,
			ObjectID: commit2NotesBlob,
		},
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	subtreeID := repo.Add(Object{
		Type: object.TypeTree,
		Data: subtreeData,
	})

	commit1Notes := []byte("Hello, World!\n")
	commit1NotesBlob := repo.Add(Object{
		Type: object.TypeBlob,
		Data: commit1Notes,
	})
	rootData, err := (object.Tree{
		{
			Name:     "85be80f2ff7482f53f6581be8683036d7a59c0e7",
			Mode:     object.ModePlain,
			ObjectID: commit1NotesBlob,
		},
		{
			Name:     "af",
			Mode:     object.ModeDir,
			ObjectID: subtreeID,
		},
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	root := repo.Add(Object{
		Type: object.TypeTree,
		Data: rootData,
	})

	tests := []struct {
		name   string
		commit githash.SHA1
		want   []byte
	}{
		{
			name:   "Root",
			commit: commit1,
			want:   commit1Notes,
		},
		{
			name:   "Subdir",
			commit: commit2,
			want:   commit2Notes,
		},
		{
			name:   "NotFound",
			commit: commit3,
			want:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			got := new(bytes.Buffer)
			if err := NotesForCommit(ctx, repo, got, root, test.commit); err != nil {
				t.Error(err)
			}
			if !bytes.Equal(got.Bytes(), test.want) {
				t.Errorf("notes = %q; want %q", got, test.want)
			}
		})
	}
}
