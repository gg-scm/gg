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

package savepoint

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

// Run runs f inside a SQLite savepoint.
//
// See https://sqlite.org/lang_savepoint.html for more details.
func Run(conn *sqlite.Conn, savepointName string, f func() error) error {
	if err := sqlitex.Exec(conn, `SAVEPOINT "`+savepointName+`";`, nil); err != nil {
		return err
	}
	ferr := f()
	if ferr != nil {
		defer conn.SetInterrupt(conn.SetInterrupt(nil))
		sqlitex.Exec(conn, `ROLLBACK TO SAVEPOINT "`+savepointName+`";`, nil)
		return ferr
	}
	if err := sqlitex.Exec(conn, `RELEASE SAVEPOINT "`+savepointName+`";`, nil); err != nil {
		defer conn.SetInterrupt(conn.SetInterrupt(nil))
		sqlitex.Exec(conn, `ROLLBACK TO SAVEPOINT "`+savepointName+`";`, nil)
		return err
	}
	return nil
}

// ReadOnly runs f inside a SQLite savepoint that always rolls back.
//
// See https://sqlite.org/lang_savepoint.html for more details.
func ReadOnly(conn *sqlite.Conn, savepointName string, f func() error) error {
	if err := sqlitex.Exec(conn, `SAVEPOINT "`+savepointName+`";`, nil); err != nil {
		return err
	}
	ferr := f()
	defer conn.SetInterrupt(conn.SetInterrupt(nil))
	sqlitex.Exec(conn, `ROLLBACK TO SAVEPOINT "`+savepointName+`";`, nil)
	return ferr
}
