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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gittool"
)

func main() {
	pctx, err := osProcessContext()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gg:", err)
		os.Exit(1)
	}
	err = run(context.Background(), pctx, os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if isUsage(err) {
			os.Exit(64)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, pctx *processContext, args []string) error {
	const synopsis = "gg [options] COMMAND [ARG [...]]"
	const description = "Git like Mercurial\n\n" +
		"basic commands:\n" +
		"  add           " + addSynopsis + "\n" +
		"  branch        " + branchSynopsis + "\n" +
		"  cat           " + catSynopsis + "\n" +
		"  clone         " + cloneSynopsis + "\n" +
		"  commit        " + commitSynopsis + "\n" +
		"  diff          " + diffSynopsis + "\n" +
		"  init          " + initSynopsis + "\n" +
		"  log           " + logSynopsis + "\n" +
		"  merge         " + mergeSynopsis + "\n" +
		"  pull          " + pullSynopsis + "\n" +
		"  push          " + pushSynopsis + "\n" +
		"  remove        " + removeSynopsis + "\n" +
		"  revert        " + revertSynopsis + "\n" +
		"  status        " + statusSynopsis + "\n" +
		"  update        " + updateSynopsis + "\n" +
		"\nadvanced commands:\n" +
		"  evolve        " + evolveSynopsis + "\n" +
		"  gerrithook    " + gerrithookSynopsis + "\n" +
		"  histedit      " + histeditSynopsis + "\n" +
		"  mail          " + mailSynopsis + "\n" +
		"  rebase        " + rebaseSynopsis + "\n" +
		"  upstream      " + upstreamSynopsis

	globalFlags := flag.NewFlagSet(false, synopsis, description)
	gitPath := globalFlags.String("git", "", "`path` to git executable")
	showArgs := globalFlags.Bool("show-git", false, "log git invocations")
	versionFlag := globalFlags.Bool("version", false, "display version information")
	if err := globalFlags.Parse(args); flag.IsHelp(err) {
		globalFlags.Help(pctx.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if globalFlags.NArg() == 0 && !*versionFlag {
		globalFlags.Help(pctx.stdout)
		return nil
	}
	if *gitPath == "" {
		var err error
		*gitPath, err = pctx.lookPath("git")
		if err != nil {
			return fmt.Errorf("gg: %v", err)
		}
	}
	opts := gittool.Options{
		Env:    pctx.env,
		Stdin:  pctx.stdin,
		Stdout: pctx.stdout,
		Stderr: pctx.stderr,
	}
	if *showArgs {
		opts.LogHook = func(_ context.Context, args []string) {
			var buf bytes.Buffer
			buf.WriteString("gg: exec: git")
			for _, a := range args {
				buf.WriteByte(' ')
				if strings.IndexByte(a, ' ') == -1 {
					buf.WriteString(a)
				} else {
					buf.WriteByte('"')
					buf.WriteString(a)
					buf.WriteByte('"')
				}
			}
			buf.WriteByte('\n')
			pctx.stderr.Write(buf.Bytes())
		}
	}
	git, err := gittool.New(*gitPath, pctx.dir, &opts)
	if err != nil {
		return fmt.Errorf("gg: %v", err)
	}
	cc := &cmdContext{
		dir:    pctx.dir,
		git:    git,
		stdout: pctx.stdout,
		stderr: pctx.stderr,
	}
	if *versionFlag {
		if err := showVersion(ctx, cc); isUsage(err) {
			return err
		} else if err != nil {
			return fmt.Errorf("gg: %v", err)
		}
	}
	err = dispatch(ctx, cc, globalFlags, globalFlags.Arg(0), globalFlags.Args()[1:])
	if isUsage(err) {
		return err
	}
	if err != nil {
		return fmt.Errorf("gg: %v", err)
	}
	return nil
}

type cmdContext struct {
	dir string

	git *gittool.Tool

	stdout io.Writer
	stderr io.Writer
}

func (cc *cmdContext) abs(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(cc.dir, path)
}

func (cc *cmdContext) withDir(path string) *cmdContext {
	path = cc.abs(path)
	return &cmdContext{
		dir:    path,
		git:    cc.git.WithDir(path),
		stdout: cc.stdout,
		stderr: cc.stderr,
	}
}

func dispatch(ctx context.Context, cc *cmdContext, globalFlags *flag.FlagSet, name string, args []string) error {
	switch name {
	case "add":
		return add(ctx, cc, args)
	case "branch":
		return branch(ctx, cc, args)
	case "cat":
		return cat(ctx, cc, args)
	case "clone":
		return clone(ctx, cc, args)
	case "commit", "ci":
		return commit(ctx, cc, args)
	case "diff":
		return diff(ctx, cc, args)
	case "evolve":
		return evolve(ctx, cc, args)
	case "gerrithook":
		return gerrithook(ctx, cc, args)
	case "histedit":
		return histedit(ctx, cc, args)
	case "init":
		return init_(ctx, cc, args)
	case "log", "history":
		return log(ctx, cc, args)
	case "mail":
		return mail(ctx, cc, args)
	case "merge":
		return merge(ctx, cc, args)
	case "pull":
		return pull(ctx, cc, args)
	case "push":
		return push(ctx, cc, args)
	case "remove", "rm":
		return remove(ctx, cc, args)
	case "rebase":
		return rebase(ctx, cc, args)
	case "revert":
		return revert(ctx, cc, args)
	case "status", "st", "check":
		return status(ctx, cc, args)
	case "update", "up", "checkout", "co":
		return update(ctx, cc, args)
	case "upstream":
		return upstream(ctx, cc, args)
	case "version":
		return showVersion(ctx, cc)
	case "help":
		if len(args) == 0 {
			globalFlags.Help(cc.stdout)
			return nil
		}
		if len(args) > 1 || strings.HasPrefix(args[0], "-") {
			return usagef("help [command]")
		}
		return dispatch(ctx, cc, globalFlags, args[0], []string{"--help"})
	case "ez":
		f := flag.NewFlagSet(true, "gg ez [-re=0]", "")
		re := f.Bool("re", true, "rematch")
		f.Parse(args)
		if *re {
			fmt.Fprintln(cc.stdout, "lol")
		} else {
			fmt.Fprintln(cc.stdout, ":(")
		}
		return nil
	default:
		return usagef("unknown command %s", name)
	}
}

// Build information filled in at link time (see -X link flag).
var (
	// versionInfo is a human-readable version number like "1.0.0".
	versionInfo = ""

	// buildCommit is the full hex-formatted hash of the commit that the
	// build came from, optionally ending with a plus if the source had
	// local modifications.
	buildCommit = ""

	// buildTime is the time the build started in RFC 3339 format.
	buildTime = ""
)

func showVersion(ctx context.Context, cc *cmdContext) error {
	commit := buildCommit
	localMods := strings.HasSuffix(buildCommit, "+")
	if localMods {
		commit = commit[:len(commit)-1]
	}
	var err error
	switch {
	case versionInfo != "" && buildTime != "":
		_, err = fmt.Fprintf(cc.stdout, "gg version %s, built on %s\n", versionInfo, buildTime)
	case versionInfo != "" && buildTime == "":
		_, err = fmt.Fprintf(cc.stdout, "gg version %s\n", versionInfo)
	case versionInfo == "" && commit != "" && localMods && buildTime != "":
		_, err = fmt.Fprintf(cc.stdout, "gg built from source at %s on %s with local modifications\n", commit, buildTime)
	case versionInfo == "" && commit != "" && !localMods && buildTime != "":
		_, err = fmt.Fprintf(cc.stdout, "gg built from source at %s on %s\n", commit, buildTime)
	case versionInfo == "" && commit != "" && localMods && buildTime == "":
		_, err = fmt.Fprintf(cc.stdout, "gg built from source at %s with local modifications\n", commit)
	case versionInfo == "" && commit != "" && !localMods && buildTime == "":
		_, err = fmt.Fprintf(cc.stdout, "gg built from source at %s\n", commit)
	default:
		_, err = fmt.Fprintln(cc.stdout, "gg built from source")
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cc.stdout, "go: %s %s %s/%s\n", runtime.Version(), runtime.Compiler, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	return cc.git.RunInteractive(ctx, "--version")
}

type processContext struct {
	dir string
	env []string

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	lookPath func(string) (string, error)
}

func osProcessContext() (*processContext, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &processContext{
		dir:      dir,
		env:      os.Environ(),
		stdin:    os.Stdin,
		stdout:   os.Stdout,
		stderr:   os.Stderr,
		lookPath: exec.LookPath,
	}, nil
}

type usageError string

func usagef(format string, args ...interface{}) error {
	e := usageError(fmt.Sprintf(format, args...))
	return &e
}

func (ue *usageError) Error() string {
	return "gg: usage: " + string(*ue)
}

func isUsage(e error) bool {
	_, ok := e.(*usageError)
	return ok
}
