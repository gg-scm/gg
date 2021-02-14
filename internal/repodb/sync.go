// Copyright 2021 The gg Authors
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

package repodb

import (
	"bufio"
	"context"
	"crypto/sha1"
	"fmt"
	"io"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"gg-scm.io/pkg/git/githash"
	"gg-scm.io/pkg/git/object"
	"gg-scm.io/pkg/git/packfile"
	"gg-scm.io/pkg/git/packfile/client"
	"zombiezen.com/go/bass/sql/sqlitefile"
)

// Sync copies any new commits from the Git repository into the database then
// writes back to the repository any newly added "refs/gg/rev/XX" refs.
func Sync(ctx context.Context, conn *sqlite.Conn, gitDir string) (err error) {
	remote, err := client.NewRemote(client.URLFromPath(gitDir), nil)
	if err != nil {
		return fmt.Errorf("index commits: %w", err)
	}
	stream, err := remote.StartPull(ctx)
	if err != nil {
		return fmt.Errorf("index commits: %w", err)
	}
	defer stream.Close()
	refs, err := stream.ListRefs("HEAD", "refs/heads/", "refs/tags/")
	if err != nil {
		return fmt.Errorf("index commits: %w", err)
	}

	defer conn.SetInterrupt(conn.SetInterrupt(ctx.Done()))
	defer sqlitex.Save(conn)(&err)
	req := new(client.PullRequest)
	req.Want, err = findNewCommits(conn, refs)
	if err != nil {
		return fmt.Errorf("index commits: %w", err)
	}
	if len(req.Want) == 0 {
		// Up to date! Nothing to do.
		return nil
	}
	// TODO(soon): Populate Have based on repository contents.
	if stream.Capabilities().Has(client.PullCapFilter) {
		req.Filter = "tree:0"
	}
	resp, err := stream.Negotiate(req)
	if err != nil {
		return fmt.Errorf("index commits: %w", err)
	}
	if err := sqlitefile.ExecScript(conn, sqlFiles, "sync/init.sql", nil); err != nil {
		return fmt.Errorf("index commits: %w", err)
	}
	defer func() {
		cleanupErr := sqlitefile.ExecScript(conn, sqlFiles, "sync/cleanup.sql", nil)
		if cleanupErr != nil && err == nil {
			err = fmt.Errorf("index commits: %w", cleanupErr)
		}
	}()
	err = unpack(conn, packfile.NewReader(bufio.NewReader(resp.Packfile)))
	resp.Packfile.Close()
	if err != nil {
		return fmt.Errorf("index commits: %w", err)
	}
	// TODO(soon): We don't need the stream past here.

	if err := undeltify(conn); err != nil {
		return fmt.Errorf("index commits: %w", err)
	}
	if err := copyPackCommits(conn); err != nil {
		return fmt.Errorf("index commits: %w", err)
	}
	if err := copyPackTags(conn); err != nil {
		return fmt.Errorf("index commits: %w", err)
	}
	return nil
}

func findNewCommits(conn *sqlite.Conn, refs map[githash.Ref]*client.Ref) ([]githash.SHA1, error) {
	hasObjectStmt, err := sqlitefile.PrepareTransient(conn, sqlFiles, "sync/has_ref_object.sql")
	if err != nil {
		return nil, fmt.Errorf("find new commits: %w", err)
	}
	defer hasObjectStmt.Finalize()
	hashSet := make(map[githash.SHA1]struct{}, len(refs))
	for _, ref := range refs {
		hasObjectStmt.SetBytes(":sha1sum", ref.ObjectID[:])
		hasRow, err := hasObjectStmt.Step()
		if err != nil {
			return nil, fmt.Errorf("find new commits: %w", err)
		}
		if !hasRow {
			return nil, fmt.Errorf("find new commits: missing data from query")
		}
		if inDB := hasObjectStmt.ColumnInt(0) != 0; !inDB {
			hashSet[ref.ObjectID] = struct{}{}
		}
		if err := hasObjectStmt.Reset(); err != nil {
			return nil, fmt.Errorf("find new commits: %w", err)
		}
	}
	hashList := make([]githash.SHA1, 0, len(hashSet))
	for h := range hashSet {
		hashList = append(hashList, h)
	}
	return hashList, nil
}

func unpack(conn *sqlite.Conn, p *packfile.Reader) (err error) {
	files := [...]string{
		"sync/insert_non_delta.sql",
		"sync/insert_offset_delta.sql",
		"sync/insert_ref_delta.sql",
		"sync/set_pack_object_sum.sql",
	}
	stmts := make([]*sqlite.Stmt, 0, len(files))
	defer func() {
		for _, stmt := range stmts {
			stmt.Finalize()
		}
	}()
	for _, fname := range files {
		s, err := sqlitefile.PrepareTransient(conn, sqlFiles, fname)
		if err != nil {
			return fmt.Errorf("unpack: %w", err)
		}
		stmts = append(stmts, s)
	}
	insertNonDeltaStmt := stmts[0]
	insertOffsetDeltaStmt := stmts[1]
	insertRefDeltaStmt := stmts[2]
	setObjectSumStmt := stmts[3]

	for {
		hdr, err := p.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("unpack: %w", err)
		}
		switch hdr.Type {
		case packfile.Commit, packfile.Tag:
			err := insertPackObject(conn, insertNonDeltaStmt, setObjectSumStmt, hdr, p)
			if err != nil {
				return fmt.Errorf("unpack: %w", err)
			}
		case packfile.OffsetDelta:
			insertOffsetDeltaStmt.SetInt64(":offset", hdr.Offset)
			insertOffsetDeltaStmt.SetInt64(":base_offset", hdr.BaseOffset)
			insertOffsetDeltaStmt.SetInt64(":size", hdr.Size)
			if _, err := insertOffsetDeltaStmt.Step(); err != nil {
				return fmt.Errorf("unpack: %w", err)
			}
			if err := insertOffsetDeltaStmt.Reset(); err != nil {
				return fmt.Errorf("unpack: %w", err)
			}
			if err := writeDelta(conn, hdr.Offset, p); err != nil {
				return fmt.Errorf("unpack: %w", err)
			}
		case packfile.RefDelta:
			insertRefDeltaStmt.SetInt64(":offset", hdr.Offset)
			insertRefDeltaStmt.SetBytes(":base_object", hdr.BaseObject[:])
			insertRefDeltaStmt.SetInt64(":size", hdr.Size)
			if _, err := insertRefDeltaStmt.Step(); err != nil {
				return fmt.Errorf("unpack: %w", err)
			}
			if err := insertRefDeltaStmt.Reset(); err != nil {
				return fmt.Errorf("unpack: %w", err)
			}
			if err := writeDelta(conn, hdr.Offset, p); err != nil {
				return fmt.Errorf("unpack: %w", err)
			}
		}
	}
}

func insertPackObject(conn *sqlite.Conn, insertObject, setObjectSum *sqlite.Stmt, hdr *packfile.Header, r io.Reader) (err error) {
	defer sqlitex.Save(conn)(&err)

	prefix := object.Prefix{
		Type: hdr.Type.NonDelta(),
		Size: hdr.Size,
	}
	prefixData, err := prefix.MarshalBinary()
	if err != nil {
		return fmt.Errorf("unpack object: %w", err)
	}
	insertObject.SetInt64(":offset", hdr.Offset)
	insertObject.SetInt64(":type", int64(hdr.Type))
	insertObject.SetInt64(":size", hdr.Size)
	if _, err := insertObject.Step(); err != nil {
		return fmt.Errorf("unpack %v: %w", prefix.Type, err)
	}
	if err := insertObject.Reset(); err != nil {
		return fmt.Errorf("unpack %v: %w", prefix.Type, err)
	}

	blob, err := conn.OpenBlob("temp", "pack_objects", "data", hdr.Offset, true)
	if err != nil {
		return fmt.Errorf("unpack %v: %w", prefix.Type, err)
	}
	h := sha1.New()
	h.Write(prefixData)
	writeSize, copyErr := io.Copy(io.MultiWriter(blob, h), r)
	closeErr := blob.Close()
	if copyErr != nil {
		return fmt.Errorf("unpack %v: %w", prefix.Type, copyErr)
	}
	if writeSize != hdr.Size {
		return fmt.Errorf("unpack %v: data less than stated size", prefix.Type)
	}
	if closeErr != nil {
		return fmt.Errorf("unpack %v: %w", prefix.Type, closeErr)
	}

	var sum githash.SHA1
	h.Sum(sum[:0])
	setObjectSum.SetBytes(":sha1sum", sum[:])
	setObjectSum.SetInt64(":offset", hdr.Offset)
	if _, err := setObjectSum.Step(); err != nil {
		return fmt.Errorf("unpack %v: %w", prefix.Type, err)
	}
	if err := setObjectSum.Reset(); err != nil {
		return fmt.Errorf("unpack %v: %w", prefix.Type, err)
	}
	return nil
}

func writeDelta(conn *sqlite.Conn, offset int64, r io.Reader) error {
	blob, err := conn.OpenBlob("temp", "pack_deltas", "delta_data", offset, true)
	if err != nil {
		return fmt.Errorf("save delta: %w", err)
	}
	_, copyErr := io.Copy(blob, r)
	closeErr := blob.Close()
	if copyErr != nil {
		return fmt.Errorf("save delta: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("save delta: %w", closeErr)
	}
	return nil
}

func undeltify(conn *sqlite.Conn) error {
	files := [...]string{
		"sync/find_delta.sql",
		"sync/insert_non_delta.sql",
		"sync/set_pack_object_sum.sql",
		"sync/sweep_delta.sql",
	}
	stmts := make([]*sqlite.Stmt, 0, len(files))
	defer func() {
		for _, stmt := range stmts {
			stmt.Finalize()
		}
	}()
	for _, fname := range files {
		s, err := sqlitefile.PrepareTransient(conn, sqlFiles, fname)
		if err != nil {
			return fmt.Errorf("undeltify: %w", err)
		}
		stmts = append(stmts, s)
	}
	findStmt := stmts[0]
	insertNonDeltaStmt := stmts[1]
	setObjectSumStmt := stmts[2]
	sweepStmt := stmts[3]

	for ; ; findStmt.Reset() {
		reconstructed := false
		for {
			hasRow, err := findStmt.Step()
			if err != nil {
				return fmt.Errorf("undeltify: %w", err)
			}
			if !hasRow {
				break
			}
			reconstructed = true

			typ := packfile.ObjectType(findStmt.GetInt64("type"))
			baseOffset := findStmt.GetInt64("base_offset")
			deltaOffset := findStmt.GetInt64("delta_offset")
			baseBlob, err := conn.OpenBlob("temp", "pack_objects", "data", baseOffset, false)
			if err != nil {
				return fmt.Errorf("undeltify: open base: %w", err)
			}
			deltaBlob, err := conn.OpenBlob("temp", "pack_deltas", "delta_data", deltaOffset, false)
			if err != nil {
				baseBlob.Close()
				return fmt.Errorf("undeltify: open delta: %w", err)
			}
			r := packfile.NewDeltaReader(baseBlob, bufio.NewReader(deltaBlob))
			hdr := &packfile.Header{
				Offset: deltaOffset,
				Type:   typ,
			}
			hdr.Size, err = r.Size()
			if err != nil {
				baseBlob.Close()
				deltaBlob.Close()
				return fmt.Errorf("undeltify: %w", err)
			}
			err = insertPackObject(conn, insertNonDeltaStmt, setObjectSumStmt, hdr, r)
			deltaBlob.Close()
			baseBlob.Close()
			if err != nil {
				return fmt.Errorf("undeltify: %w", err)
			}
		}
		if !reconstructed {
			break
		}
		if _, err := sweepStmt.Step(); err != nil {
			return fmt.Errorf("reconstruct delta blobs: %w", err)
		}
		sweepStmt.Reset()
	}
	return nil
}

func copyPackCommits(conn *sqlite.Conn) (err error) {
	defer sqlitex.Save(conn)(&err)
	var data []byte
	var sha1Hash githash.SHA1
	var commit object.Commit
	parents := make(map[githash.SHA1][]githash.SHA1)
	fillParents := func(stmt *sqlite.Stmt) error {
		if n := stmt.ColumnLen(0); cap(data) < n {
			data = make([]byte, n)
		} else {
			data = data[:n]
		}
		stmt.ColumnBytes(0, data)
		stmt.ColumnBytes(1, sha1Hash[:])
		if err := commit.UnmarshalBinary(data); err != nil {
			return err
		}
		parents[sha1Hash] = commit.Parents
		return nil
	}
	err = sqlitex.ExecTransient(conn,
		`SELECT "data", "sha1sum" FROM "pack_objects" WHERE "type" = ?;`,
		fillParents, int(packfile.Commit))
	if err != nil {
		return err
	}
	for _, commitToAdd := range sortCommits(parents) {
		readCommit := func(stmt *sqlite.Stmt) error {
			if n := stmt.ColumnLen(0); cap(data) < n {
				data = make([]byte, n)
			} else {
				data = data[:n]
			}
			stmt.ColumnBytes(0, data)
			stmt.ColumnBytes(1, sha1Hash[:])
			return commit.UnmarshalBinary(data)
		}
		err := sqlitex.Exec(conn,
			`SELECT "data" FROM "pack_objects" WHERE "sha1sum" = ? AND "type" = ?;`,
			readCommit, commitToAdd[:], packfile.Commit)
		if err != nil {
			return err
		}
		_, err = InsertCommit(conn, &commit)
		if IsExist(err) {
			continue
		}
		if err != nil {
			return err
		}
	}
	// TODO(now): Add tags.
	return nil
}

func copyPackTags(conn *sqlite.Conn) (err error) {
	defer sqlitex.Save(conn)(&err)
	insertTag, err := sqlitefile.PrepareTransient(conn, sqlFiles, "sync/insert_tag.sql")
	if err != nil {
		return fmt.Errorf("insert tags: %w", err)
	}
	defer insertTag.Finalize()
	var data []byte
	var sha1Sum githash.SHA1
	var tag object.Tag
	insertTags := func(stmt *sqlite.Stmt) error {
		if n := stmt.ColumnLen(0); cap(data) < n {
			data = make([]byte, n)
		} else {
			data = data[:n]
		}
		stmt.ColumnBytes(0, data)
		stmt.ColumnBytes(1, sha1Sum[:])
		if err := tag.UnmarshalBinary(data); err != nil {
			return fmt.Errorf("insert tag %v: %w", sha1Sum, err)
		}
		insertTag.SetBytes(":sha1sum", sha1Sum[:])
		insertTag.SetBytes(":object_sha1sum", tag.ObjectID[:])
		insertTag.SetText(":object_type", string(tag.ObjectType))
		insertTag.SetText(":name", tag.Name)
		insertTag.SetText(":tagger", string(tag.Tagger))
		insertTag.SetText(":tag_date", tag.Time.UTC().Format(sqliteTimestampFormat))
		_, tzOffset := tag.Time.Zone()
		insertTag.SetInt64(":tag_tzoffset", int64(tzOffset))
		insertTag.SetText(":message", tag.Message)
		if _, err := insertTag.Step(); err != nil {
			return fmt.Errorf("insert tag %v (%s): %w", sha1Sum, tag.Name, err)
		}
		if err := insertTag.Reset(); err != nil {
			return err
		}
		return nil
	}
	err = sqlitex.ExecTransient(conn,
		`SELECT "data", "sha1sum" FROM "pack_objects" WHERE "type" = ?;`,
		insertTags, int(packfile.Tag))
	if err != nil {
		return err
	}
	return nil
}

func sortCommits(graph map[githash.SHA1][]githash.SHA1) []githash.SHA1 {
	list := make([]githash.SHA1, 0, len(graph))
	visited := make(map[githash.SHA1]struct{}, len(graph))
	var stack []githash.SHA1
	for curr := range graph {
		stack = append(stack, curr)
		for len(stack) > 0 {
			stack, curr = stack[:len(stack)-1], stack[len(stack)-1]
			if _, v := visited[curr]; v {
				continue
			}

			// Push parents to stack in reverse order, so first parent is visited first.
			hasUnvisitedParents := false
			parents, inGraph := graph[curr]
			if !inGraph {
				visited[curr] = struct{}{}
				continue
			}
			for i := len(parents) - 1; i >= 0; i-- {
				p := parents[i]
				if _, v := visited[p]; v {
					continue
				}
				if !hasUnvisitedParents {
					// Revisit this node after visiting all parents.
					stack = append(stack, curr)
					hasUnvisitedParents = true
				}
				stack = append(stack, p)
			}
			if hasUnvisitedParents {
				continue
			}

			// Add commit to list.
			list = append(list, curr)
			visited[curr] = struct{}{}
		}
	}
	return list
}
