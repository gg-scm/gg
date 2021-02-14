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
	"fmt"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"gg-scm.io/pkg/git/object"
	"zombiezen.com/go/bass/sql/sqlitefile"
)

// InsertCommit creates a new commit from the given information.
func InsertCommit(conn *sqlite.Conn, c *object.Commit) (revno int64, err error) {
	defer sqlitex.Save(conn)(&err)
	exists := false
	commitSHA1 := c.SHA1()
	err = sqlitex.Exec(conn, `SELECT "revno" FROM "commits" WHERE "sha1sum" = ?;`, func(stmt *sqlite.Stmt) error {
		revno = stmt.ColumnInt64(0)
		exists = true
		return nil
	}, commitSHA1[:])
	if exists {
		return revno, fmt.Errorf("insert commit %v: %w", commitSHA1, errObjectExists)
	}
	revno, err = nextRevno(conn)
	if err != nil {
		return -1, fmt.Errorf("insert commit %v: %w", commitSHA1, err)
	}
	authorTime := c.AuthorTime.UTC().Format(sqliteTimestampFormat)
	_, authorTZOffset := c.AuthorTime.Zone()
	commitTime := c.CommitTime.UTC().Format(sqliteTimestampFormat)
	_, commitTZOffset := c.CommitTime.Zone()
	err = sqlitefile.Exec(conn, sqlFiles, "commit/insert.sql", &sqlitefile.ExecOptions{
		Named: map[string]interface{}{
			":revno":           revno,
			":sha1sum":         commitSHA1[:],
			":tree_sha1sum":    c.Tree[:],
			":message":         c.Message,
			":author":          string(c.Author),
			":author_date":     authorTime,
			":author_tzoffset": authorTZOffset,
			":committer":       string(c.Committer),
			":commit_date":     commitTime,
			":commit_tzoffset": commitTZOffset,
			":gpg_signature":   string(c.GPGSignature),
		},
	})
	if err != nil {
		return -1, fmt.Errorf("insert commit %v: %w", commitSHA1, err)
	}
	for i, par := range c.Parents {
		err := sqlitefile.Exec(conn, sqlFiles, "commit/insert_parent.sql", &sqlitefile.ExecOptions{
			Named: map[string]interface{}{
				":revno":          revno,
				":index":          i,
				":parent_sha1sum": par[:],
			},
		})
		if err != nil {
			return -1, fmt.Errorf("insert commit %v: %w", commitSHA1, err)
		}
	}
	return revno, nil
}

func nextRevno(conn *sqlite.Conn) (int64, error) {
	var next int64
	err := sqlitex.Exec(conn, `select coalesce((select max("revno") + 1 from "commits"), 0);`, func(stmt *sqlite.Stmt) error {
		next = stmt.ColumnInt64(0)
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("get next revision number: %w", err)
	}
	return next, nil
}
