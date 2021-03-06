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

// Package repodb provides a SQLite-based commit index.
package repodb

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"zombiezen.com/go/bass/sql/sqlitemigration"
)

//go:embed *.sql
//go:embed commit/*.sql
//go:embed revision/*.sql
//go:embed sync/*.sql
var sqlFiles embed.FS

// Create opens the database for the given Git common directory, creating it if
// necessary.
func Create(ctx context.Context, gitDir string) (*sqlite.Conn, error) {
	return open(ctx, gitDir, sqlite.SQLITE_OPEN_CREATE)
}

// Open opens the database for the given Git common directory. If there is no
// available database, Open returns an error for which IsMissingDatabase returns
// true.
func Open(ctx context.Context, gitDir string) (*sqlite.Conn, error) {
	return open(ctx, gitDir, 0)
}

func open(ctx context.Context, gitDir string, mode sqlite.OpenFlags) (*sqlite.Conn, error) {
	schema, err := initSchema()
	if err != nil {
		return nil, fmt.Errorf("open commit index: %w", err)
	}
	conn, err := sqlite.OpenConn(filepath.Join(gitDir, "gg.sqlite"),
		mode,
		sqlite.SQLITE_OPEN_READWRITE,
		sqlite.SQLITE_OPEN_WAL,
		sqlite.SQLITE_OPEN_NOMUTEX,
	)
	if mode == 0 && sqlite.ErrCode(err)&0xff == sqlite.SQLITE_CANTOPEN {
		return nil, fmt.Errorf("open commit index: %w", errMissingDatabase)
	}
	if err != nil {
		return nil, fmt.Errorf("open commit index: %w", err)
	}
	conn.SetInterrupt(ctx.Done())
	if err := sqlitex.ExecTransient(conn, `PRAGMA foreign_keys = on;`, nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("open commit index: %w", err)
	}
	if err := sqlitemigration.Migrate(ctx, conn, schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("open commit index: %w", err)
	}
	return conn, nil
}

var schema struct {
	once sync.Once
	sqlitemigration.Schema
	err error
}

// initSchema returns the schema for the SQLite database.
func initSchema() (sqlitemigration.Schema, error) {
	schema.once.Do(func() {
		filenames := []string{
			"schema01.sql",
			"schema02.sql",
		}
		schema.AppID = 0x18302f95
		schema.Migrations = make([]string, 0, len(filenames))
		for _, fname := range filenames {
			source, err := readString(sqlFiles, fname)
			if err != nil {
				schema.err = err
				return
			}
			schema.Migrations = append(schema.Migrations, source)
		}
		schema.RepeatableMigration, schema.err = readString(sqlFiles, "repeatable.sql")
	})
	return schema.Schema, schema.err
}

func readString(fsys fs.FS, filename string) (string, error) {
	f, err := fsys.Open(filename)
	if err != nil {
		return "", err
	}
	content := new(strings.Builder)
	_, err = io.Copy(content, f)
	f.Close()
	if err != nil {
		return "", fmt.Errorf("%s: %w", filename, err)
	}
	return content.String(), nil
}

var (
	errMissingDatabase = errors.New("commit index not found")
	errObjectExists    = errors.New("object exists")
)

// IsMissingDatabase reports whether an error returned from Open indicates that
// a database has not been created.
func IsMissingDatabase(e error) bool {
	return errors.Is(e, errMissingDatabase)
}

// IsExist reports whether the error indicates that the object already exists.
func IsExist(e error) bool {
	return errors.Is(e, errObjectExists)
}

func execTransient(stmt *sqlite.Stmt) error {
	if _, err := stmt.Step(); err != nil {
		return err
	}
	if err := stmt.Reset(); err != nil {
		return err
	}
	return nil
}

// sqliteTimestampFormat is the date string layout used in SQLite, suitable for
// use with time.Format and time.Parse.
//
// See https://sqlite.org/lang_datefunc.html
const sqliteTimestampFormat = "2006-01-02 15:04:05"

// ParseTime converts a SQLite timestamp and timezone offset (in seconds) into
// a time.Time value. The timestamp string is assumed to be in UTC.
func ParseTime(s string, tzOffset int) (time.Time, error) {
	t, err := time.Parse(sqliteTimestampFormat, s)
	if err != nil {
		return time.Time{}, err
	}
	tzHours := tzOffset / (60 * 60)
	tzMinutes := (tzOffset / 60) % 60
	if tzOffset < 0 {
		tzMinutes = (-tzOffset / 60) % 60
	}
	loc := time.FixedZone(fmt.Sprintf("%+02d%02d", tzHours, tzMinutes), tzOffset)
	return t.In(loc), nil
}
