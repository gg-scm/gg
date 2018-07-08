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
	"errors"
	"fmt"

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gittool"
	"zombiezen.com/go/gg/internal/singleclose"
	"zombiezen.com/go/gg/internal/terminal"
)

const statusSynopsis = "show changed files in the working directory"

func status(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg status [FILE [...]]", statusSynopsis+`

aliases: st, check`)
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	var (
		addedColor     []byte
		modifiedColor  []byte
		removedColor   []byte
		missingColor   []byte
		untrackedColor []byte
		unmergedColor  []byte
	)
	cfg, err := gittool.ReadConfig(ctx, cc.git)
	if err != nil {
		return err
	}
	colorize, err := cfg.ColorBool("color.ggstatus", terminal.IsTerminal(cc.stdout))
	if err != nil {
		fmt.Fprintln(cc.stderr, "gg:", err)
	} else if colorize {
		addedColor, err = cfg.Color("color.ggstatus.added", "green")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
		modifiedColor, err = cfg.Color("color.ggstatus.modified", "blue")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
		removedColor, err = cfg.Color("color.ggstatus.removed", "red")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
		missingColor, err = cfg.Color("color.ggstatus.deleted", "cyan")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
		untrackedColor, err = cfg.Color("color.ggstatus.unknown", "magenta")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
		unmergedColor, err = cfg.Color("color.ggstatus.unmerged", "blue")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
	}
	st, err := gittool.Status(ctx, cc.git, f.Args())
	if err != nil {
		return err
	}
	stClose := singleclose.For(st)
	defer stClose.Close()
	if colorize {
		if err := terminal.ResetTextStyle(cc.stdout); err != nil {
			return err
		}
	}
	foundUnrecognized := false
	for st.Scan() {
		ent := st.Entry()
		switch {
		case ent.Code().IsModified():
			_, err = fmt.Fprintf(cc.stdout, "%sM %s\n", modifiedColor, ent.Name())
		case ent.Code().IsAdded():
			_, err = fmt.Fprintf(cc.stdout, "%sA %s\n", addedColor, ent.Name())
		case ent.Code().IsRemoved():
			_, err = fmt.Fprintf(cc.stdout, "%sR %s\n", removedColor, ent.Name())
		case ent.Code().IsCopied():
			if _, err := fmt.Fprintf(cc.stdout, "%sA %s\n", addedColor, ent.Name()); err != nil {
				return err
			}
			if colorize {
				if err := terminal.ResetTextStyle(cc.stdout); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintf(cc.stdout, "  %s\n", ent.From())
		case ent.Code().IsRenamed():
			fmt.Fprintf(cc.stdout, "%sA %s\n", addedColor, ent.Name())
			if colorize {
				if err := terminal.ResetTextStyle(cc.stdout); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintf(cc.stdout, "  %s\n%sR %s\n", ent.From(), removedColor, ent.From())
		case ent.Code().IsMissing():
			_, err = fmt.Fprintf(cc.stdout, "%s! %s\n", missingColor, ent.Name())
		case ent.Code().IsUntracked():
			_, err = fmt.Fprintf(cc.stdout, "%s? %s\n", untrackedColor, ent.Name())
		case ent.Code().IsUnmerged():
			_, err = fmt.Fprintf(cc.stdout, "%sU %s\n", unmergedColor, ent.Name())
		default:
			fmt.Fprintf(cc.stderr, "gg: unrecognized status for %s: '%v'\n", ent.Name(), ent.Code())
			foundUnrecognized = true
		}
		if err != nil {
			return err
		}
		if colorize {
			if err := terminal.ResetTextStyle(cc.stdout); err != nil {
				return err
			}
		}
	}
	if err := st.Err(); err != nil {
		return err
	}
	if err := st.Close(); err != nil {
		return err
	}
	if foundUnrecognized {
		return errors.New("unrecognized output from git status. Please file a bug at https://github.com/zombiezen/gg/issues/new and include the output from this command.")
	}
	return nil
}
