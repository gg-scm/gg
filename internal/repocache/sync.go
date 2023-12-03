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
	"bufio"
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"gg-scm.io/pkg/git/githash"
	"gg-scm.io/pkg/git/object"
	"gg-scm.io/pkg/git/packfile"
	"gg-scm.io/pkg/git/packfile/client"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitefile"
	"zombiezen.com/go/sqlite/sqlitex"
)

// CopyFrom caches any objects from the remote not present in the cache.
func (c *Cache) CopyFrom(ctx context.Context, remote *client.Remote) (err error) {
	stream, err := remote.StartPull(ctx)
	if err != nil {
		return fmt.Errorf("cache git data: %v", err)
	}
	defer stream.Close()

	refs, err := stream.ListRefs(string(githash.Head), "refs/heads/", "refs/tags/")
	if err != nil {
		return fmt.Errorf("cache git data: %v", err)
	}

	req := new(client.PullRequest)
	// TODO(soon): Fill in req.Have.
	if stream.Capabilities().Has(client.PullCapFilter) {
		req.Filter = "blob:none"
	}
	for _, ref := range refs {
		req.Want = append(req.Want, ref.ObjectID)
	}
	if len(req.Want) == 0 {
		return nil
	}
	resp, err := stream.Negotiate(req)
	if err != nil {
		return fmt.Errorf("cache git data: %v", err)
	}
	defer resp.Packfile.Close()

	c.conn.SetInterrupt(ctx.Done())
	defer c.conn.SetInterrupt(nil)
	endFn, err := sqlitex.ImmediateTransaction(c.conn)
	if err != nil {
		return fmt.Errorf("cache git data: %v", err)
	}
	defer endFn(&err)

	contentsBuf, err := sqlitefile.NewBufferSize(c.conn, 32<<10) // 32 KiB
	if err != nil {
		return err
	}
	defer contentsBuf.Close()

	r := packfile.NewReader(bufio.NewReader(resp.Packfile))
	h := sha1.New()
	var prefixBuf []byte
	var sumBuf githash.SHA1
	var newCommits []githash.SHA1
	insertStmt, err := sqlitex.PrepareTransientFS(c.conn, sqlFiles, "objects/insert.sql")
	if err != nil {
		return fmt.Errorf("cache git data: %v", err)
	}
	defer insertStmt.Finalize()

	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		var tp object.Type
		switch hdr.Type {
		case packfile.Commit:
			tp = object.TypeCommit
		case packfile.Tree:
			tp = object.TypeTree
		case packfile.Tag:
			tp = object.TypeTag
		default:
			continue
		}

		h.Reset()
		prefixBuf = object.AppendPrefix(prefixBuf[:0], tp, hdr.Size)
		h.Write(prefixBuf)
		contentsBuf.Reset()
		zw := zlib.NewWriter(contentsBuf)
		if _, err := io.Copy(io.MultiWriter(h, zw), r); err != nil {
			return err
		}
		if err := zw.Close(); err != nil {
			return err
		}
		h.Sum(sumBuf[:0])

		isNew, err := insertObject(c.conn, insertStmt, sumBuf, tp, hdr.Size, contentsBuf.Len(), contentsBuf)
		if err != nil {
			return err
		}
		if isNew && tp == object.TypeCommit {
			newCommits = append(newCommits, sumBuf)
		}
	}

	for _, id := range newCommits {
		if err := indexCommit(c.conn, id); err != nil {
			// TODO(soon): Failing to index should not abort the entire operation.
			return err
		}
	}

	return nil
}

func insertObject(conn *sqlite.Conn, insertStmt *sqlite.Stmt, name githash.SHA1, tp object.Type, uncompressedSize, compressedSize int64, compressedReader io.Reader) (inserted bool, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cache %s %v: %v", tp, name, err)
		}
	}()
	defer sqlitex.Save(conn)(&err)

	insertStmt.SetBytes(":sha1", name[:])
	insertStmt.SetText(":type", string(tp))
	insertStmt.SetInt64(":uncompressed_size", uncompressedSize)
	insertStmt.SetInt64(":compressed_size", compressedSize)
	inserted, err = insertStmt.Step()
	if err != nil {
		return false, err
	}
	var oid int64
	if inserted {
		oid = insertStmt.GetInt64("oid")
	}
	if err := insertStmt.Reset(); err != nil {
		return false, err
	}
	if !inserted {
		return false, nil
	}

	contentCol, err := conn.OpenBlob("", objectsTable, contentColumn, oid, true)
	if err != nil {
		return false, err
	}
	_, copyErr := io.Copy(contentCol, compressedReader)
	closeErr := contentCol.Close()
	if copyErr != nil {
		return false, copyErr
	}
	if closeErr != nil {
		return false, closeErr
	}
	return true, nil
}

func indexCommit(conn *sqlite.Conn, id githash.SHA1) (err error) {
	defer sqlitex.Save(conn)(&err)

	buf := new(bytes.Buffer)
	oid, tp, err := cat(conn, buf, id)
	if err != nil {
		return fmt.Errorf("index commit %v: %v", id, err)
	}
	if tp != object.TypeCommit {
		return fmt.Errorf("index commit %v: not a commit (found %v)", id, tp)
	}
	parsed, err := object.ParseCommit(buf.Bytes())
	if err != nil {
		return fmt.Errorf("index commit %v: %v", id, err)
	}

	err = sqlitex.ExecuteScriptFS(conn, sqlFiles, "commits/upsert.sql", &sqlitex.ExecOptions{
		Named: map[string]any{
			":oid":                  oid,
			":tree":                 parsed.Tree[:],
			":parents":              jsonSHA1Array(parsed.Parents),
			":author":               string(parsed.Author),
			":author_timestamp":     parsed.AuthorTime.Unix(),
			":author_tzoffset_mins": tzOffsetMinutes(parsed.AuthorTime),
			":committer":            string(parsed.Committer),
			":commit_timestamp":     parsed.CommitTime.Unix(),
			":commit_tzoffset_mins": tzOffsetMinutes(parsed.CommitTime),
			":message":              parsed.Message,
		},
	})
	if err != nil {
		return fmt.Errorf("index commit %v: %v", id, err)
	}

	return nil
}

func tzOffsetMinutes(t time.Time) int {
	_, off := t.Zone()
	return off / 60
}

func jsonSHA1Array(s []githash.SHA1) string {
	sb := new(strings.Builder)
	sb.WriteString("[")
	enc := hex.NewEncoder(sb)
	for i, id := range s {
		if i > 0 {
			sb.WriteString(`,`)
		}
		sb.WriteString(`"`)
		enc.Write(id[:])
		sb.WriteString(`"`)
	}
	sb.WriteString("]")
	return sb.String()
}

const syncPageSize = 32 << 10 // 32 KiB
