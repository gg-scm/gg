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
	"path/filepath"
	"strings"

	"zombiezen.com/go/gg/internal/flag"
)

const cloneSynopsis = "make a copy of an existing repository"

func clone(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg clone [-u=0] [-b BRANCH] SOURCE [DEST]", cloneSynopsis)
	branch := f.String("b", "HEAD", "`branch` to check out")
	f.Alias("b", "branch")
	checkout := f.Bool("u", true, "check out a working directory")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() == 0 {
		return usagef("must pass clone source")
	}
	if f.NArg() > 2 {
		return usagef("can't pass more than one destination")
	}
	if *branch != "HEAD" && *checkout == false {
		return usagef("can't pass -b with -u=0")
	}
	src, dst := f.Arg(0), f.Arg(1)
	if dst == "" {
		dst = defaultCloneDest(src)
	}
	if !*checkout {
		return cc.git.RunInteractive(ctx, "clone", "--bare", "--", src, dst)
	}
	if err := cc.git.RunInteractive(ctx, "clone", "--bare", "--", src, filepath.Join(dst, ".git")); err != nil {
		return err
	}
	git := cc.git.WithDir(cc.abs(dst))
	if err := git.Run(ctx, "config", "--local", "--bool", "core.bare", "false"); err != nil {
		return err
	}
	return git.Run(ctx, "checkout", *branch, "--")
}

func defaultCloneDest(url string) string {
	if strings.HasSuffix(url, "/.git") {
		url = url[:len(url)-5]
	} else if strings.HasSuffix(url, ".git") {
		url = url[:len(url)-4]
	}
	if i := strings.LastIndexByte(url, '/'); i != -1 {
		return url[i:]
	}
	return url
}
