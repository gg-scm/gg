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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/flag"
)

const revertSynopsis = "restore files to their checkout state"

func revert(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg revert [-r REV] [--all] [--no-backup] [FILE [...]]", revertSynopsis+`

	With no revision specified, revert the specified files or directories
	to the contents they had at HEAD.
	
	Modified files are saved with a .orig suffix before reverting. To
	disable these backups, use `+"`--no-backup`.")
	all := f.Bool("all", false, "revert all changes when no arguments given")
	noBackups := f.Bool("C", false, "do not save backup copies of files")
	f.Alias("C", "no-backup")
	rev := f.String("r", git.Head.String(), "revert to specified `rev`ision")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() == 0 && !*all {
		return usagef("no arguments given.  Use -all to revert entire repository.")
	}

	revObj, err := cc.git.ParseRev(ctx, *rev)
	if err != nil {
		if *rev == git.Head.String() {
			// If HEAD fails to parse (empty repo), then just use reset.
			rmArgs := []string{"reset", "--"}
			for _, f := range f.Args() {
				rmArgs = append(rmArgs, git.LiteralPath(f).String())
			}
			return cc.git.Run(ctx, rmArgs...)
		}
		return err
	}

	// Check whether files are known to Git or exist in the working tree.
	var unknowns []int
	for i, arg := range f.Args() {
		if _, err := os.Stat(cc.abs(arg)); err != nil {
			unknowns = append(unknowns, i)
		}
	}
	if len(unknowns) > 0 {
		unknownPathspecs := make([]git.Pathspec, 0, len(unknowns))
		for _, i := range unknowns {
			unknownPathspecs = append(unknownPathspecs, git.LiteralPath(f.Arg(i)))
		}
		workRoot, err := cc.git.WorkTree(ctx)
		if err != nil {
			return err
		}
		tree, err := cc.git.ListTree(ctx, revObj.Commit.String(), git.ListTreeOptions{
			NameOnly:  true,
			Recursive: true,
			Pathspecs: unknownPathspecs,
		})
		if err != nil {
			return err
		}
		for _, i := range unknowns {
			arg := f.Arg(i)
			argPath, err := worktreeRelativePath(cc, workRoot, arg)
			if err != nil {
				return err
			}
			if _, ok := tree[argPath]; !ok {
				return fmt.Errorf("%q is not known to gg", arg)
			}
		}
	}

	// Find the list of files that have changed between the revision and
	// the working tree.
	var pathspecs []git.Pathspec
	for _, f := range f.Args() {
		pathspecs = append(pathspecs, git.LiteralPath(f))
	}
	st, err := cc.git.DiffStatus(ctx, git.DiffStatusOptions{
		Commit1:        revObj.Commit.String(),
		Pathspecs:      pathspecs,
		DisableRenames: true,
	})
	if err != nil {
		return err
	}
	var adds, deletes, mods, chmods []git.Pathspec
	for _, ent := range st {
		switch ent.Code {
		case git.DiffStatusAdded:
			adds = append(adds, ent.Name.Pathspec())
		case git.DiffStatusDeleted:
			deletes = append(deletes, ent.Name.Pathspec())
		case git.DiffStatusModified:
			mods = append(mods, ent.Name.Pathspec())
		case git.DiffStatusChangedMode:
			chmods = append(chmods, ent.Name.Pathspec())
		}
	}

	// Find the list of files that need to be backed up: these are
	// modified locally beyond what's in HEAD.
	if !*noBackups {
		if err := backupForRevert(ctx, cc, mods); err != nil {
			return err
		}
	}

	// Now revert files.
	if len(adds) > 0 {
		// TODO(#59): Can be fully removed if no local modifications (add test).
		if err := cc.git.Remove(ctx, adds, git.RemoveOptions{KeepWorkingCopy: true}); err != nil {
			return err
		}
	}
	if len(mods)+len(chmods)+len(deletes) > 0 {
		coArgs := []string{"checkout", revObj.Commit.String(), "--"}
		for _, f := range mods {
			coArgs = append(coArgs, f.String())
		}
		for _, f := range chmods {
			coArgs = append(coArgs, f.String())
		}
		for _, f := range deletes {
			coArgs = append(coArgs, f.String())
		}
		if err := cc.git.Run(ctx, coArgs...); err != nil {
			return err
		}
	}
	return nil
}

// backupForRevert creates ".orig" files for any modified files that
// have local modifications.
func backupForRevert(ctx context.Context, cc *cmdContext, modified []git.Pathspec) error {
	if len(modified) == 0 {
		return nil
	}
	st, err := cc.git.Status(ctx, git.StatusOptions{
		DisableRenames: true,
		Pathspecs:      modified,
	})
	if err != nil {
		return fmt.Errorf("backing up files: %w", err)
	}
	var names []git.TopPath
	for _, ent := range st {
		names = append(names, ent.Name)
	}
	if len(names) == 0 {
		// Nothing to back up.
		return nil
	}

	top, err := cc.git.WorkTree(ctx)
	if err != nil {
		return fmt.Errorf("backing up files: %w", err)
	}
	for _, name := range names {
		path := filepath.Join(top, filepath.FromSlash(name.String()))
		if err := os.Rename(path, path+".orig"); err != nil {
			return fmt.Errorf("backing up files: %w", err)
		}
	}
	return nil
}

// worktreeRelativePath converts a working tree file reference to a top path.
// TODO(soon): Move this into internal/git.
func worktreeRelativePath(cc *cmdContext, worktree string, name string) (git.TopPath, error) {
	a, err := evalSymlinksSloppy(cc.abs(name))
	if err != nil {
		return "", err
	}
	prefix := worktree + string(filepath.Separator)
	if !strings.HasPrefix(a, prefix) {
		return "", fmt.Errorf("%s is not under %s", name, worktree)
	}
	return git.TopPath(filepath.ToSlash(a[len(prefix):])), nil
}

// evalSymlinksSloppy is like filepath.EvalSymlinks, but permits the file itself
// to be a symlink or not exist.
func evalSymlinksSloppy(path string) (string, error) {
	orig := path
	suffix := filepath.Base(path)
	path = filepath.Dir(path)
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return filepath.Join(resolved, suffix), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("eval symlinks for %q: %w", orig, err)
		}
		next := filepath.Dir(path)
		if next == "" || strings.HasSuffix(next, string(filepath.Separator)) {
			return "", fmt.Errorf("eval symlinks for %q: %w", orig, err)
		}
		suffix = filepath.Join(filepath.Base(path), suffix)
		path = next
	}
}
