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
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"crawshaw.io/sqlite"
	"gg-scm.io/pkg/git/githash"
	"gg-scm.io/tool/internal/savepoint"
	"zombiezen.com/go/bass/sql/sqlitefile"
)

// Revision is a parsed reference to a single commit.
type Revision struct {
	Revno int64
	SHA1  githash.SHA1
	Ref   githash.Ref
}

// ParseRevision finds the commit named by rev inside the index.
func ParseRevision(ctx context.Context, conn *sqlite.Conn, rev string) (*Revision, error) {
	if rev == "" {
		return nil, fmt.Errorf("parse revision %q: %w", rev, errNotExist)
	}
	defer conn.SetInterrupt(conn.SetInterrupt(ctx.Done()))
	var r *Revision
	err := savepoint.ReadOnly(conn, "parse_revision", func() error {
		var err error
		r, err = parseRevision(conn, rev)
		return err
	})
	return r, err
}

func parseRevision(conn *sqlite.Conn, rev string) (*Revision, error) {
	parsers := []func(*sqlite.Conn, string) (*Revision, error){
		findTipRevision,
		findRevisionByNumber, // Try decimal integers as revision numbers first.
		findSHA1Revision,
		findRefRevision,
	}
	for _, f := range parsers {
		r, err := f(conn, rev)
		if err == nil {
			return r, nil
		}
		if !IsNotExist(err) {
			return nil, fmt.Errorf("parse revision %q: %w", rev, err)
		}
	}
	return nil, fmt.Errorf("parse revision %q: %w", rev, errNotExist)
}

func findTipRevision(conn *sqlite.Conn, rev string) (*Revision, error) {
	if rev != "tip" {
		return nil, errNotExist
	}
	var r *Revision
	err := sqlitefile.Exec(conn, sqlFiles, "revision/find_tip.sql", &sqlitefile.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			r = new(Revision)
			r.Revno = stmt.GetInt64("revno")
			stmt.GetBytes("sha1sum", r.SHA1[:])
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errNotExist
	}
	return r, nil
}

func findSHA1Revision(conn *sqlite.Conn, rev string) (*Revision, error) {
	hexRev := upperHex(rev)
	if hexRev == "" {
		return nil, errNotExist
	}
	lower, upper := hexRange(hexRev)
	params := map[string]interface{}{
		":hex_rev":       hexRev,
		":lower_sha1sum": lower,
		":upper_sha1sum": nil,
	}
	if upper != nil {
		params[":upper_sha1sum"] = upper
	}
	n := 0
	r := new(Revision)
	err := sqlitefile.Exec(conn, sqlFiles, "revision/find_sha1.sql", &sqlitefile.ExecOptions{
		Named: params,
		ResultFunc: func(stmt *sqlite.Stmt) error {
			n++
			if n > 1 {
				return errors.New("multiple revisions for identifier")
			}
			r.Revno = stmt.GetInt64("revno")
			stmt.GetBytes("sha1sum", r.SHA1[:])
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errNotExist
	}
	return r, nil
}

func findRevisionByNumber(conn *sqlite.Conn, rev string) (*Revision, error) {
	if strings.HasPrefix(rev, "0") && rev != "0" {
		// Reject zero-padded decimal numbers.
		return nil, errNotExist
	}
	revno, err := strconv.ParseInt(rev, 10, 64)
	if err != nil {
		return nil, errNotExist
	}
	var r *Revision
	err = sqlitefile.Exec(conn, sqlFiles, "revision/find_number.sql", &sqlitefile.ExecOptions{
		Named: map[string]interface{}{":revno": revno},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			r = new(Revision)
			r.Revno = revno
			stmt.GetBytes("sha1sum", r.SHA1[:])
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errNotExist
	}
	return r, nil
}

func findRefRevision(conn *sqlite.Conn, rev string) (*Revision, error) {
	if rev == "@" || rev == "." {
		rev = githash.Head.String()
	}
	var r *Revision
	err := sqlitefile.Exec(conn, sqlFiles, "revision/find_ref.sql", &sqlitefile.ExecOptions{
		Named: map[string]interface{}{":ref": rev},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			name := githash.Ref(stmt.GetText("name"))
			if r != nil {
				if r.Ref.String() == rev {
					// Use exact match over an imprecise one.
					return nil
				}
				return fmt.Errorf("%q is ambiguous (found %q and %q)", rev, r.Ref, name)
			}
			r = new(Revision)
			r.Revno = stmt.GetInt64("revno")
			stmt.GetBytes("sha1sum", r.SHA1[:])
			r.Ref = name
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errNotExist
	}
	return r, nil
}

// ListRefs lists all of the refs in the repository with tags dereferenced.
func ListRefs(ctx context.Context, conn *sqlite.Conn) (map[githash.Ref]githash.SHA1, error) {
	defer conn.SetInterrupt(conn.SetInterrupt(ctx.Done()))
	refs := make(map[githash.Ref]githash.SHA1)
	err := sqlitefile.Exec(conn, sqlFiles, "revision/list_refs.sql", &sqlitefile.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			var sum githash.SHA1
			stmt.GetBytes("sha1sum", sum[:])
			refs[githash.Ref(stmt.GetText("name"))] = sum
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list refs: %w", err)
	}
	return refs, nil
}

// upperHex converts s into an uppercase hex string.
// It returns the empty string if s is not a valid hex string.
func upperHex(s string) string {
	sb := new(strings.Builder)
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case '0' <= c && c <= '9' || 'A' <= c && c <= 'F':
			sb.WriteByte(c)
		case 'a' <= c && c <= 'f':
			sb.WriteByte(c - 'a' + 'A')
		default:
			return ""
		}
	}
	return sb.String()
}

// hexRange returns the range of BLOBs that could match the given hex prefix.
// lower is inclusive and upper is exclusive. If upper is nil, then there is
// no upper bound. Lower is nil if and only if the hex prefix is empty.
func hexRange(s string) (lower, upper []byte) {
	for i := 0; i+1 < len(s); i += 2 {
		hi := dehexDigit(s[i])
		lo := dehexDigit(s[i+1])
		b := hi<<4 | lo
		lower = append(lower, b)
		upper = append(upper, b)
	}
	if len(s)%2 == 0 {
		// Ended on whole hex digit
		if !incBlob(upper) {
			upper = nil
		}
	} else {
		// Only high bits available for last byte
		hi := dehexDigit(s[len(s)-1])
		lower = append(lower, hi<<4)
		if hi == 0xf {
			if len(upper) == 0 || !incBlob(upper) {
				upper = nil
			} else {
				upper = append(upper, 0)
			}
		} else {
			upper = append(upper, (hi+1)<<4)
		}
	}
	return
}

// incBlob adds one to the big-endian unsigned integer in b.
// It returns false if the result overflows.
func incBlob(b []byte) (ok bool) {
	carry := byte(0)
	for i := len(b) - 1; i >= 0; i-- {
		x := b[i] + carry + 1
		if x > b[i] {
			carry = 0
		} else {
			carry = 1
		}
		b[i] = x
	}
	return carry == 0
}

func dehexDigit(c byte) uint8 {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'A' <= c && c <= 'F':
		return c - 'A' + 0xa
	case 'a' <= c && c <= 'f':
		return c - 'a' + 0xa
	default:
		panic("invalid hex digit")
	}

}

// IsNotExist reports whether the error indicates that an object does not exist.
func IsNotExist(e error) bool {
	return errors.Is(e, errNotExist)
}

var errNotExist = errors.New("object does not exist")
