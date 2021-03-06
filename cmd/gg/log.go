// Copyright 2018 The gg Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"gg-scm.io/pkg/git/githash"
	"gg-scm.io/pkg/git/object"
	"gg-scm.io/tool/internal/flag"
	"gg-scm.io/tool/internal/repodb"
	"zombiezen.com/go/bass/sql/sqlitefile"
)

const logSynopsis = "show revision history of entire repository or files"

type logFlags struct {
	follow      bool
	followFirst bool
	graph       bool
	rev         []string
	reverse     bool
	stat        bool
}

func log(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg log [OPTION [...]] [FILE]", logSynopsis+`

aliases: history`)
	flags := new(logFlags)
	f.BoolVar(&flags.follow, "follow", false, "follow file history across copies and renames")
	f.BoolVar(&flags.followFirst, "follow-first", false, "only follow the first parent of merge commits")
	f.BoolVar(&flags.graph, "graph", false, "show the revision DAG")
	f.Alias("graph", "G")
	f.MultiStringVar(&flags.rev, "r", "show the specified `rev`ision or range")
	f.BoolVar(&flags.reverse, "reverse", false, "reverse order of commits")
	f.BoolVar(&flags.stat, "stat", false, "include diffstat-style summary of each commit")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 1 {
		return usagef("only one file allowed")
	}
	file := f.Arg(0)
	if file != "" || flags.followFirst || flags.graph || flags.stat {
		// If any unsupported options are given, fall back to `git log`.
		return logWithGit(ctx, cc, flags, file)
	}

	dir, err := cc.git.GitDir(ctx)
	if err != nil {
		return err
	}
	db, err := repodb.Open(ctx, dir)
	if repodb.IsMissingDatabase(err) {
		return logWithGit(ctx, cc, flags, file)
	} else if err != nil {
		return err
	} else {
		defer db.Close()
		return logWithDB(ctx, cc, flags, dir, db)
	}
}

func logWithGit(ctx context.Context, cc *cmdContext, flags *logFlags, file string) error {
	var logArgs []string
	logArgs = append(logArgs, "log", "--decorate=auto", "--date-order")
	if flags.follow {
		logArgs = append(logArgs, "--follow")
	}
	if flags.followFirst {
		logArgs = append(logArgs, "--first-parent")
	}
	if flags.graph {
		logArgs = append(logArgs, "--graph")
	}
	if flags.reverse {
		logArgs = append(logArgs, "--reverse")
	}
	if flags.stat {
		logArgs = append(logArgs, "--stat")
	}
	for _, r := range flags.rev {
		if strings.HasPrefix(r, "-") {
			return usagef("revisions must not start with '-'")
		}
	}
	if len(flags.rev) == 0 {
		logArgs = append(logArgs, "--all")
	} else {
		logArgs = append(logArgs, flags.rev...)
	}
	logArgs = append(logArgs, "--")
	if file != "" {
		logArgs = append(logArgs, file)
	}
	return cc.interactiveGit(ctx, logArgs...)
}

func logWithDB(ctx context.Context, cc *cmdContext, flags *logFlags, dir string, db *sqlite.Conn) (err error) {
	if err := sqlitex.ExecTransient(db, "BEGIN;", nil); err != nil {
		return err
	}
	if err := repodb.Sync(ctx, db, dir); err != nil {
		defer db.SetInterrupt(db.SetInterrupt(nil))
		sqlitex.ExecTransient(db, "ROLLBACK;", nil)
		return err
	}
	// The rest of the function is read-only. We don't want a failure later to
	// drop the results of the sync, but we also want the rest of the reads to
	// occur within the same transaction.
	defer func() {
		if commitErr := sqlitex.ExecTransient(db, "COMMIT;", nil); commitErr != nil {
			defer db.SetInterrupt(db.SetInterrupt(nil))
			sqlitex.ExecTransient(db, "ROLLBACK;", nil)
			if err == nil {
				err = commitErr
			}
		}
	}()

	var revnos []int64
	if len(flags.rev) > 0 {
		for _, r := range flags.rev {
			parsed, err := repodb.ParseRevision(ctx, db, r)
			if err != nil {
				return err
			}
			revnos = append(revnos, parsed.Revno)
		}
	} else {
		err := sqlitex.Exec(db, `select max("revno") from "commits";`, func(stmt *sqlite.Stmt) error {
			max := stmt.ColumnInt64(0)
			for i := int64(0); i <= max; i++ {
				revnos = append(revnos, i)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if flags.reverse {
		sort.Slice(revnos, func(i, j int) bool {
			return revnos[i] < revnos[j]
		})
	} else {
		sort.Slice(revnos, func(i, j int) bool {
			return revnos[i] > revnos[j]
		})
	}
	// TODO(soon): Remove duplicates.

	for _, revno := range revnos {
		buf := new(bytes.Buffer)
		err := sqlitefile.Exec(db, sqlFiles, "log.sql", &sqlitefile.ExecOptions{
			Named: map[string]interface{}{
				":revno": revno,
			},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				var id githash.SHA1
				stmt.GetBytes("sha1sum", id[:])
				author := object.User(stmt.GetText("author"))
				authorDate, err := repodb.ParseTime(stmt.GetText("author_date"), int(stmt.GetInt64("author_tzoffset")))
				if err != nil {
					return err
				}
				message := stmt.GetText("message")
				summary := message
				if i := strings.Index(message, "\n"); i != -1 {
					summary = message[:i]
				}

				buf.Reset()
				fmt.Fprintf(buf, "\x1b[33mcommit:      %d:%x\x1b[0m\n", revno, id[:6])
				err = sqlitefile.Exec(db, sqlFiles, "log_labels.sql", &sqlitefile.ExecOptions{
					Named: map[string]interface{}{
						":revno": revno,
					},
					ResultFunc: func(stmt *sqlite.Stmt) error {
						ref := githash.Ref(stmt.GetText("name"))
						if ref == githash.Head {
							return nil
						}
						if branch := ref.Branch(); branch != "" {
							fmt.Fprintf(buf, "branch:      %s\n", branch)
						} else if tag := ref.Tag(); tag != "" {
							fmt.Fprintf(buf, "tag:         %s\n", tag)
						} else {
							fmt.Fprintf(buf, "label:       %s\n", ref)
						}
						return nil
					},
				})
				if err != nil {
					return err
				}
				// TODO(now): labels
				fmt.Fprintf(buf, "author:      %s\n", author)
				fmt.Fprintf(buf, "date:        %s\n", authorDate.Format("Mon Jan 02 15:04:05 2006 -0700"))
				fmt.Fprintf(buf, "summary:     %s\n", summary)
				buf.WriteString("\n")
				if _, err := cc.stdout.Write(buf.Bytes()); err != nil {
					return err
				}
				return nil
			},
		})
		if err != nil {
			return fmt.Errorf("print revision %d: %w", revno, err)
		}
	}

	return nil
}
