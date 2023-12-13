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
	"fmt"
	"io"
	"strings"

	"gg-scm.io/pkg/git/githash"
	"gg-scm.io/pkg/git/object"
)

// NotesForCommit reads the notes for a particular commit and root reference.
// If there are no notes for the given ID,
// nothing will be written to dst and NotesForCommit will return nil.
func NotesForCommit(ctx context.Context, cat Catter, dst io.Writer, notesRef githash.SHA1, id githash.SHA1) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("read notes for %v: %v", id, err)
		}
	}()

	buf := new(bytes.Buffer)
	var prefix strings.Builder
	rest := id.String()
	for curr := notesRef; ; {
		if err := cat.Cat(ctx, buf, object.TypeTree, curr); err != nil {
			return err
		}
		currTree, err := object.ParseTree(buf.Bytes())
		if err != nil {
			return err
		}
		buf.Reset()
		if ent := currTree.Search(rest); ent != nil {
			// Notes file found.
			if !ent.Mode.IsRegular() {
				return fmt.Errorf("%s%s: not a regular file", prefix.String(), rest)
			}
			// TODO(maybe): Prevent tags to blobs?
			return cat.Cat(ctx, dst, object.TypeBlob, ent.ObjectID)
		}

		dir := rest[:2]
		rest = rest[2:]
		ent := currTree.Search(dir)
		if ent == nil {
			return nil
		}
		if ent.Mode != object.ModeDir {
			return fmt.Errorf("%s%s: not a directory", prefix.String(), dir)
		}
		prefix.WriteString(dir)
		prefix.WriteString("/")
		curr = ent.ObjectID
	}
}
