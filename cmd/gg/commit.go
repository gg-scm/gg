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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gittool"
)

const commitSynopsis = "commit the specified files or all outstanding changes"

func commit(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg commit [--amend] [-m MSG] [FILE [...]]", commitSynopsis+`

aliases: ci`)
	amend := f.Bool("amend", false, "amend the parent of the working directory")
	msg := f.String("m", "", "use text as commit `message`")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	// git does not always operate correctly on specified files when
	// running from a subdirectory (see https://github.com/zombiezen/gg/issues/10).
	// To work around, we always run commit from the top directory.
	topBytes, err := cc.git.RunOneLiner(ctx, '\n', "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	top := string(topBytes)
	var commitArgs []string
	commitArgs = append(commitArgs, "commit", "--quiet")
	if *amend {
		commitArgs = append(commitArgs, "--amend")
	}
	if *msg != "" {
		commitArgs = append(commitArgs, "--message="+*msg)
	}
	if f.NArg() > 0 {
		// Commit specific files.
		commitArgs = append(commitArgs, "--")
		commitArgs = append(commitArgs, f.Args()...)
		if err := toPathspecs(cc.dir, top, commitArgs[len(commitArgs)-f.NArg():]); err != nil {
			return err
		}
	} else if exists, err := cc.git.Query(ctx, "cat-file", "-e", "MERGE_HEAD"); err == nil && exists {
		// Merging: must not provide selective files.
		commitArgs = append(commitArgs, "-a")
	} else {
		// Commit all tracked files without modifying index.
		commitArgs = append(commitArgs, "--")
		fileStart := len(commitArgs)
		var err error
		commitArgs, err = inferCommitFiles(ctx, cc.git, commitArgs)
		if err != nil {
			return err
		}
		if len(commitArgs) == fileStart && !*amend {
			return errors.New("nothing changed")
		}
	}
	return cc.git.WithDir(top).RunInteractive(ctx, commitArgs...)
}

// toPathspecs rewrites the given set of arguments into either absolute
// path specs or top-level path specs.
func toPathspecs(wd, top string, files []string) error {
	var err error
	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		return fmt.Errorf("find current directory: %v", err)
	}
	top, err = filepath.EvalSymlinks(top)
	if err != nil {
		return fmt.Errorf("find top directory: %v", err)
	}
	for i := range files {
		if !filepath.IsAbs(files[i]) {
			files[i] = filepath.Join(wd, files[i])
		}
		if files[i] == top {
			files[i] = ":(top,literal)"
		} else if strings.HasPrefix(files[i], top+string(filepath.Separator)) {
			// Prepend pathspec options to interpret relative to top of
			// repository and ignore globs. See gitglossary(7) for more details.
			files[i] = ":(top,literal)" + files[i][len(top)+len(string(filepath.Separator)):]
		} else {
			files[i] = ":(literal)" + files[i]
		}
	}
	return nil
}

func inferCommitFiles(ctx context.Context, git *gittool.Tool, files []string) ([]string, error) {
	missing, missingStaged, unmerged := 0, 0, 0
	p, err := git.Start(ctx, "status", "--porcelain", "-z", "-unormal")
	if err != nil {
		return files, err
	}
	defer p.Wait()
	r := bufio.NewReader(p)
	filesStart := len(files)
	for {
		ent, err := readStatusEntry(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return files[:filesStart], err
		}
		switch {
		case ent.isMissing():
			missing++
			if ent.code[0] != ' ' {
				missingStaged++
			}
		case ent.isAdded() || ent.isModified() || ent.isRemoved() || ent.isCopied():
			// Prepend pathspec options to interpret relative to top of
			// repository and ignore globs. See gitglossary(7) for more details.
			files = append(files, ":(top,literal)"+ent.name)
		case ent.isRenamed():
			files = append(files, ":(top,literal)"+ent.name, ":(top,literal)"+ent.from)
		case ent.isIgnored() || ent.isUntracked():
			// Skip
		case ent.isUnmerged():
			unmerged++
		default:
			panic("unhandled status code")
		}
	}
	if unmerged == 1 {
		return files[:filesStart], errors.New("1 unmerged file; see 'gg status'")
	}
	if unmerged > 1 {
		return files[:filesStart], fmt.Errorf("%d unmerged files; see 'gg status'", unmerged)
	}
	if len(files) == filesStart {
		switch missing {
		case 0:
			return files[:filesStart], nil
		case 1:
			return files[:filesStart], errors.New("nothing changed (1 missing file; see 'gg status')")
		default:
			return files[:filesStart], fmt.Errorf("nothing changed (%d missing files; see 'gg status')", missing)
		}
	}
	if missingStaged == 1 {
		return files[:filesStart], errors.New("git has staged changes for 1 missing file; see 'gg status'")
	}
	if missingStaged > 1 {
		return files[:filesStart], fmt.Errorf("git has staged changes for %d missing file; see 'gg status'", missingStaged)
	}
	return files, p.Wait()
}
