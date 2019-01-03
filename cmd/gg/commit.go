// Copyright 2018 Google LLC
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
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"unicode"

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/git"
)

const commitSynopsis = "commit the specified files or all outstanding changes"

func commit(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg commit [--amend] [-m MSG] [FILE [...]]", commitSynopsis+`

aliases: ci

	Commits changes to the given files into the repository. If no files
	are given, then all changes reported by `+"`gg status`"+` will be
	committed.

	Unlike Git, gg does not require you to stage your changes into the
	index. This approximates the behavior of `+"`git commit -a`"+`, but
	this command will only change the index if the commit succeeds.`)
	amend := f.Bool("amend", false, "amend the parent of the working directory")
	msg := f.String("m", "", "use text as commit `message`")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if *msg == "" {
		// Open message in editor.
		cfg, err := cc.git.ReadConfig(ctx)
		if err != nil {
			return err
		}
		commentChar, err := cfg.CommentChar()
		if err != nil {
			return err
		}
		initial, err := commitMessageTemplate(ctx, cc.git, *amend, commentChar)
		if err != nil {
			return err
		}
		editorOut, err := cc.editor.open(ctx, "COMMIT_MSG", initial)
		if err != nil {
			return err
		}
		*msg = cleanupMessage(string(editorOut), commentChar)
	} else {
		*msg = cleanupMessage(*msg, "")
	}
	if f.NArg() > 0 {
		// Commit or amend specific files.
		var pathspecs []git.Pathspec
		for _, arg := range f.Args() {
			pathspecs = append(pathspecs, git.LiteralPath(arg))
		}
		if *amend {
			return cc.git.AmendFiles(ctx, pathspecs, git.AmendOptions{Message: *msg})
		}
		return cc.git.CommitFiles(ctx, *msg, pathspecs, git.CommitOptions{})
	}
	// Commit or amend all tracked files.
	hasChanges, err := verifyNoMissingOrUnmerged(ctx, cc.git)
	if err != nil {
		return err
	}
	if *amend {
		return cc.git.AmendAll(ctx, git.AmendOptions{Message: *msg})
	}
	if !hasChanges {
		return errors.New("nothing changed")
	}
	return cc.git.CommitAll(ctx, *msg, git.CommitOptions{})
}

func commitMessageTemplate(ctx context.Context, g *git.Git, amend bool, commentChar string) ([]byte, error) {
	var initial []byte
	if amend {
		// Use previous commit message.
		info, err := g.CommitInfo(ctx, "HEAD")
		if err != nil {
			return nil, fmt.Errorf("gather commit message template: %v", err)
		}
		initial = []byte(info.Message)
	} else if gitDir, err := g.GitDir(ctx); err == nil {
		// Opportunistically grab the merge message.
		if mergeMsg, err := ioutil.ReadFile(filepath.Join(gitDir, "MERGE_MSG")); err == nil {
			initial = mergeMsg
		}
	}
	if !bytes.HasSuffix(initial, []byte("\n")) {
		initial = append(initial, '\n')
	}
	initial = append(initial, '\n') // blank line
	initial = append(initial, commentChar...)
	initial = append(initial, " Please enter a commit message.\n"...)
	initial = append(initial, commentChar...)
	initial = append(initial, " Lines starting with '"...)
	initial = append(initial, commentChar...)
	initial = append(initial, "' will be ignored.\n"...)
	// TODO(soon): Add branch info and files to be committed.
	return initial, nil
}

func cleanupMessage(s string, commentPrefix string) string {
	lines := strings.SplitAfter(s, "\n")

	// Filter out comment lines and strip trailing whitespace.
	n := len(lines)
	lines = lines[:0]
	for _, line := range lines[:n] {
		if commentPrefix != "" && strings.HasPrefix(line, commentPrefix) {
			continue
		}
		lines = append(lines, strings.TrimRightFunc(line, unicode.IsSpace))
	}

	// Remove trailing blank lines.
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] != "" {
			break
		}
		lines = lines[:i]
	}

	// Join lines into single string.
	sb := new(strings.Builder)
	for _, line := range lines {
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func verifyNoMissingOrUnmerged(ctx context.Context, g *git.Git) (hasChanges bool, _ error) {
	missing, missingStaged, unmerged := 0, 0, 0
	st, err := g.Status(ctx, git.StatusOptions{})
	if err != nil {
		return false, err
	}
	for _, ent := range st {
		switch {
		case ent.Code.IsMissing():
			missing++
			if ent.Code[0] != ' ' {
				missingStaged++
			}
		case ent.Code.IsAdded() || ent.Code.IsModified() || ent.Code.IsRemoved() || ent.Code.IsCopied() || ent.Code.IsRenamed():
			hasChanges = true
		case ent.Code.IsUntracked():
			// Skip
		case ent.Code.IsUnmerged():
			unmerged++
		default:
			return false, fmt.Errorf("unhandled status: %v", ent)
		}
	}
	if unmerged == 1 {
		return false, errors.New("1 unmerged file; see 'gg status'")
	}
	if unmerged > 1 {
		return false, fmt.Errorf("%d unmerged files; see 'gg status'", unmerged)
	}
	if !hasChanges {
		switch missing {
		case 0:
			return false, nil
		case 1:
			return false, errors.New("nothing changed (1 missing file; see 'gg status')")
		default:
			return false, fmt.Errorf("nothing changed (%d missing files; see 'gg status')", missing)
		}
	}
	if missingStaged == 1 {
		return true, errors.New("git has staged changes for 1 missing file; see 'gg status'")
	}
	if missingStaged > 1 {
		return true, fmt.Errorf("git has staged changes for %d missing files; see 'gg status'", missingStaged)
	}
	return true, nil
}
