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

// Package repocache provides optimized queries over a Git repository
// using an on-disk index.
package repocache

import (
	"compress/zlib"
	"context"
	"crypto/sha1"
	"embed"
	"errors"
	"fmt"
	"hash"
	"io"

	"gg-scm.io/pkg/git/githash"
	"gg-scm.io/pkg/git/object"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/ext/refunc"
	"zombiezen.com/go/sqlite/sqlitex"
)

const (
	objectsTable  = "objects"
	contentColumn = "content"
)

//go:embed schema.sql
//go:embed commits/*.sql
//go:embed objects/*.sql
var sqlFiles embed.FS

const appID int32 = 0x40a9233d

const currentUserVersion = 1

// Cache represents an open connection to a cache database.
type Cache struct {
	conn *sqlite.Conn
}

// Open opens a cache file on disk, creating it if necessary.
func Open(ctx context.Context, path string) (*Cache, error) {
	conn, err := sqlite.OpenConn(path, sqlite.OpenCreate|sqlite.OpenReadWrite)
	if err != nil {
		return nil, fmt.Errorf("open git repo cache %s: %w", path, err)
	}
	if err := refunc.Register(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("open git repo cache %s: %w", path, err)
	}
	if err := sqlitex.ExecuteTransient(conn, "PRAGMA page_size = 8192;", nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("open git repo cache %s: %w", path, err)
	}

	conn.SetInterrupt(ctx.Done())
	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("open git repo cache %s: %w", path, err)
	}
	if err := sqlitex.ExecuteTransient(conn, `PRAGMA foreign_keys = on;`, nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("open git repo cache %s: %w", path, err)
	}
	conn.SetInterrupt(nil)
	return &Cache{conn}, nil
}

func migrate(conn *sqlite.Conn) (err error) {
	endFn, err := sqlitex.ImmediateTransaction(conn)
	if err != nil {
		return err
	}
	defer endFn(&err)

	gotVersion, err := ensureAppID(conn)
	if err != nil {
		return err
	}
	if gotVersion != currentUserVersion {
		if err := dropAllTables(conn); err != nil {
			return err
		}
	}
	if err := sqlitex.ExecuteScriptFS(conn, sqlFiles, "schema.sql", nil); err != nil {
		return err
	}
	userVersionStmt := fmt.Sprintf("PRAGMA user_version = %d;", currentUserVersion)
	if err := sqlitex.ExecuteTransient(conn, userVersionStmt, nil); err != nil {
		return err
	}
	return nil
}

// OpenObject returns a reader for the given object in the cache.
// If the object is not present in the cache,
// then OpenObject will return an error that wraps [ErrObjectNotFound].
// If the returned reader returns an EOF,
// it guarantees that the bytes read from it match the hash.
func (c *Cache) OpenObject(ctx context.Context, id githash.SHA1) (_ object.Prefix, _ io.ReadCloser, err error) {
	c.conn.SetInterrupt(ctx.Done())
	defer c.conn.SetInterrupt(nil)
	defer sqlitex.Transaction(c.conn)(&err)
	_, prefix, rc, err := openObject(c.conn, id)
	return prefix, rc, err
}

// Stat returns the prefix of the given object.
// If the object is not present in the cache,
// then Stat will return an error that wraps [ErrObjectNotFound].
func (c *Cache) Stat(ctx context.Context, id githash.SHA1) (_ object.Prefix, err error) {
	c.conn.SetInterrupt(ctx.Done())
	defer c.conn.SetInterrupt(nil)
	defer sqlitex.Transaction(c.conn)(&err)
	_, prefix, err := stat(c.conn, id)
	return prefix, err
}

func stat(conn *sqlite.Conn, id githash.SHA1) (oid int64, prefix object.Prefix, err error) {
	prefix.Size = -1
	err = sqlitex.ExecuteTransientFS(conn, sqlFiles, "objects/find.sql", &sqlitex.ExecOptions{
		Named: map[string]any{
			":sha1": id[:],
		},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			oid = stmt.GetInt64("oid")
			prefix.Type = object.Type(stmt.GetText("type"))
			prefix.Size = stmt.GetInt64("uncompressed_size")
			return nil
		},
	})
	if err != nil {
		return 0, object.Prefix{}, fmt.Errorf("read git object %v: %v", id, err)
	}
	if prefix.Size < 0 {
		return 0, object.Prefix{}, fmt.Errorf("read git object %v: %w", id, ErrObjectNotFound)
	}
	return oid, prefix, nil
}

// objectReader is an open handle to a Git object.
// It verifies the read content on EOF.
type objectReader struct {
	blob *sqlite.Blob
	zr   io.ReadCloser
	err  error

	id        githash.SHA1
	hash      hash.Hash
	remaining int64
}

func openObject(conn *sqlite.Conn, id githash.SHA1) (oid int64, _ object.Prefix, _ io.ReadCloser, err error) {
	oid, prefix, err := stat(conn, id)
	if err != nil {
		return 0, object.Prefix{}, nil, err
	}

	// Intentionally not holding onto a transaction or savepoint here.
	compressedContent, err := conn.OpenBlob("", objectsTable, contentColumn, oid, false)
	if err != nil {
		return oid, prefix, nil, fmt.Errorf("read git object %v: %v", id, err)
	}
	r := &objectReader{
		blob:      compressedContent,
		id:        id,
		remaining: prefix.Size,
		hash:      sha1.New(),
	}
	r.zr, err = zlib.NewReader(compressedContent)
	if err != nil {
		compressedContent.Close()
		return oid, prefix, nil, fmt.Errorf("read git object %v: %v", id, err)
	}
	r.hash.Write(object.AppendPrefix(nil, prefix.Type, prefix.Size))
	return oid, prefix, r, nil
}

func (r *objectReader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	if len(p) > 0 {
		n, err = r.zr.Read(p)
		r.hash.Write(p[:n])
		r.remaining -= int64(n)
	}
	switch {
	case err != nil && err != io.EOF:
		return n, fmt.Errorf("read git object %v: %w", r.id, err)
	case err == io.EOF && r.remaining > 0:
		return n, fmt.Errorf("read git object %v: %w", r.id, io.ErrUnexpectedEOF)
	case r.remaining == 0:
		var gotHash githash.SHA1
		r.hash.Sum(gotHash[:0])
		if gotHash != r.id {
			return n, fmt.Errorf("read git object %v: corrupted content (hash = %v)", r.id, gotHash)
		}
		r.err = io.EOF
		return n, io.EOF
	}
	return n, nil
}

func (r *objectReader) Close() error {
	r.err = fmt.Errorf("read git object %v: closed", r.id)
	r.zr.Close()
	return r.blob.Close()
}

// Close releases all resources associated with the cache connection.
func (c *Cache) Close() error {
	return c.conn.Close()
}

func dropAllTables(conn *sqlite.Conn) (err error) {
	defer sqlitex.Save(conn)(&err)

	var tables, views []string
	const query = `SELECT "type", "name" FROM sqlite_schema WHERE "type" in ('table', 'view');`
	err = sqlitex.ExecuteTransient(conn, query, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			name := stmt.ColumnText(1)
			switch stmt.ColumnText(0) {
			case "table":
				tables = append(tables, name)
			case "view":
				views = append(views, name)
			}
			return nil
		},
	})
	if err != nil {
		return fmt.Errorf("drop all tables: %w", err)
	}
	for _, name := range views {
		if err := sqlitex.ExecuteTransient(conn, `DROP VIEW "`+name+`";`, nil); err != nil {
			return fmt.Errorf("drop all tables: %w", err)
		}
	}
	for _, name := range tables {
		if err := sqlitex.ExecuteTransient(conn, `DROP TABLE "`+name+`";`, nil); err != nil {
			return fmt.Errorf("drop all tables: %w", err)
		}
	}
	return nil
}

func userVersion(conn *sqlite.Conn) (int32, error) {
	var version int32
	err := sqlitex.ExecuteTransient(conn, "PRAGMA user_version;", &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			version = stmt.ColumnInt32(0)
			return nil
		},
	})
	if err != nil {
		return 0, fmt.Errorf("get database user_version: %w", err)
	}
	return version, nil
}

func ensureAppID(conn *sqlite.Conn) (schemaVersion int32, err error) {
	defer sqlitex.Save(conn)(&err)

	var hasSchema bool
	err = sqlitex.ExecuteTransient(conn, "VALUES ((SELECT COUNT(*) FROM sqlite_master) > 0);", &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			hasSchema = stmt.ColumnInt(0) != 0
			return nil
		},
	})
	if err != nil {
		return 0, err
	}
	var dbAppID int32
	err = sqlitex.ExecuteTransient(conn, "PRAGMA application_id;", &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			dbAppID = stmt.ColumnInt32(0)
			return nil
		},
	})
	if err != nil {
		return 0, err
	}
	if dbAppID != appID && !(dbAppID == 0 && !hasSchema) {
		return 0, fmt.Errorf("database application_id = %#x (expected %#x)", dbAppID, appID)
	}
	schemaVersion, err = userVersion(conn)
	if err != nil {
		return 0, err
	}
	// Using Sprintf because PRAGMAs don't permit arbitrary expressions, and thus
	// don't permit using parameter substitution.
	err = sqlitex.ExecuteTransient(conn, fmt.Sprintf("PRAGMA application_id = %d;", appID), nil)
	if err != nil {
		return 0, err
	}
	return schemaVersion, nil
}

var ErrObjectNotFound = errors.New("git object not found")
