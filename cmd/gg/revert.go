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
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/gitobj"
	"gg-scm.io/pkg/internal/gittool"
	"gg-scm.io/pkg/internal/singleclose"
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
	rev := f.String("r", gitobj.Head.String(), "revert to specified `rev`ision")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() == 0 && !*all {
		return usagef("no arguments given.  Use -all to revert entire repository.")
	}

	revObj, err := gittool.ParseRev(ctx, cc.git, *rev)
	if err != nil {
		if *rev == gitobj.Head.String() {
			// If HEAD fails to parse (empty repo), then just use reset.
			rmArgs := []string{"reset", "--"}
			for _, f := range f.Args() {
				rmArgs = append(rmArgs, gittool.LiteralPath(f).String())
			}
			return cc.git.Run(ctx, rmArgs...)
		}
		return err
	}

	// Find the list of files that have changed between the revision and
	// the working tree.
	var pathspecs []gittool.Pathspec
	for _, f := range f.Args() {
		pathspecs = append(pathspecs, gittool.LiteralPath(f))
	}
	dr, err := gittool.DiffStatus(ctx, cc.git, gittool.DiffStatusOptions{
		Commit1:        revObj.Commit().String(),
		Pathspecs:      pathspecs,
		DisableRenames: true,
	})
	if err != nil {
		return err
	}
	drCloser := singleclose.For(dr)
	defer drCloser.Close()
	var adds, deletes, mods, chmods []gittool.Pathspec
	for dr.Scan() {
		switch dr.Entry().Code() {
		case gittool.DiffStatusAdded:
			adds = append(adds, dr.Entry().Name().Pathspec())
		case gittool.DiffStatusDeleted:
			deletes = append(deletes, dr.Entry().Name().Pathspec())
		case gittool.DiffStatusModified:
			mods = append(mods, dr.Entry().Name().Pathspec())
		case gittool.DiffStatusChangedMode:
			chmods = append(chmods, dr.Entry().Name().Pathspec())
		}
	}
	if err := dr.Err(); err != nil {
		return err
	}
	if err := drCloser.Close(); err != nil {
		return err
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
		rmArgs := []string{"rm", "--cached", "--"}
		for _, f := range adds {
			rmArgs = append(rmArgs, f.String())
		}
		if err := cc.git.Run(ctx, rmArgs...); err != nil {
			return err
		}
	}
	if len(mods)+len(chmods)+len(deletes) > 0 {
		coArgs := []string{"checkout", revObj.Commit().String(), "--"}
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
func backupForRevert(ctx context.Context, cc *cmdContext, modified []gittool.Pathspec) error {
	if len(modified) == 0 {
		return nil
	}
	sr, err := gittool.Status(ctx, cc.git, gittool.StatusOptions{
		DisableRenames: true,
		Pathspecs:      modified,
	})
	if err != nil {
		return fmt.Errorf("backing up files: %v", err)
	}
	srCloser := singleclose.For(sr)
	defer srCloser.Close()
	var names []gittool.TopPath
	for sr.Scan() {
		names = append(names, sr.Entry().Name())
	}
	if err := sr.Err(); err != nil {
		return fmt.Errorf("backing up files: %v", err)
	}
	if err := srCloser.Close(); err != nil {
		return fmt.Errorf("backing up files: %v", err)
	}
	if len(names) == 0 {
		// Nothing to back up.
		return nil
	}

	topBytes, err := cc.git.RunOneLiner(ctx, '\n', "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("backing up files: %v", err)
	}
	top := string(topBytes)
	for _, name := range names {
		path := filepath.Join(top, filepath.FromSlash(name.String()))
		if err := os.Rename(path, path+".orig"); err != nil {
			return fmt.Errorf("backing up files: %v", err)
		}
	}
	return nil
}

// appendLiteralPaths converts the arguments into literal pathspecs
// for Git.
func appendLiteralPaths(dst []gittool.Pathspec, files []string) []gittool.Pathspec {
	for _, f := range files {
		dst = append(dst, gittool.LiteralPath(f))
	}
	return dst
}
