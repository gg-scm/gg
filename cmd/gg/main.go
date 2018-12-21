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

// gg is Git like Mercurial.
//
// Learn more at https://gg-scm.io/
package main // import "gg-scm.io/pkg/cmd/gg"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/git"
	"gg-scm.io/pkg/internal/sigterm"
)

func main() {
	pctx, err := osProcessContext()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gg:", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(sig, sigterm.Signals()...)
	go func() {
		select {
		case <-sig:
			cancel()
		case <-done:
		}
	}()
	err = run(ctx, pctx, os.Args[1:])
	close(done)
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
		"  requestpull   " + requestPullSynopsis + "\n" +
		"  revert        " + revertSynopsis + "\n" +
		"  status        " + statusSynopsis + "\n" +
		"  update        " + updateSynopsis + "\n" +
		"\nadvanced commands:\n" +
		"  backout       " + backoutSynopsis + "\n" +
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
	opts := git.Options{
		Env: pctx.env,
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
	git, err := git.New(*gitPath, pctx.dir, &opts)
	if err != nil {
		return fmt.Errorf("gg: %v", err)
	}
	cc := &cmdContext{
		dir:     pctx.dir,
		xdgDirs: newXDGDirs(pctx.env),
		git:     git,
		editor: &editor{
			git:      git,
			tempRoot: pctx.tempDir,
			env:      pctx.env,
			stdin:    pctx.stdin,
			stdout:   pctx.stdout,
			stderr:   pctx.stderr,

			log: func(e error) {
				fmt.Fprintln(pctx.stderr, "gg:", e)
			},
		},
		httpClient: pctx.httpClient,
		stdin:      pctx.stdin,
		stdout:     pctx.stdout,
		stderr:     pctx.stderr,
	}
	if *versionFlag {
		if err := showVersion(ctx, cc); isUsage(err) {
			return err
		} else if err != nil {
			return fmt.Errorf("gg: %v", err)
		}
		return nil
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
	dir     string
	xdgDirs *xdgDirs

	git        *git.Git
	editor     *editor
	httpClient *http.Client

	stdin  io.Reader
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
	cc2 := new(cmdContext)
	*cc2 = *cc
	cc2.dir = cc.abs(path)
	cc2.git = cc.git.WithDir(cc2.dir)
	return cc2
}

func dispatch(ctx context.Context, cc *cmdContext, globalFlags *flag.FlagSet, name string, args []string) error {
	switch name {
	case "add":
		return add(ctx, cc, args)
	case "backout":
		return backout(ctx, cc, args)
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
	case "requestpull", "pr":
		return requestPull(ctx, cc, args)
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
	c := cc.git.Command(ctx, "--version")
	c.Stdout = cc.stdout
	c.Stderr = cc.stderr
	return sigterm.Run(ctx, c)
}

func userAgentString() string {
	if versionInfo == "" {
		return "zombiezen/gg"
	}
	return "zombiezen/gg " + versionInfo
}

// processContext is the state that gg uses to run. It is collected in
// this struct to avoid obtaining this from globals for simpler testing.
type processContext struct {
	dir     string
	env     []string
	tempDir string

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	httpClient *http.Client
	lookPath   func(string) (string, error)
}

// osProcessContext returns the default process context from global variables.
func osProcessContext() (*processContext, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &processContext{
		dir:        dir,
		tempDir:    os.TempDir(),
		env:        os.Environ(),
		stdin:      os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		httpClient: http.DefaultClient,
		lookPath:   exec.LookPath,
	}, nil
}

// getenv is like os.Getenv but reads from the given list of environment
// variables.
func getenv(environ []string, name string) string {
	// Later entries take precedence.
	for i := len(environ) - 1; i >= 0; i-- {
		e := environ[i]
		if strings.HasPrefix(e, name) && strings.HasPrefix(e[len(name):], "=") {
			return e[len(name)+1:]
		}
	}
	return ""
}

// xdgDirs implements the Free Desktop Base Directory specification for
// locating directories.
//
// The specification is at
// http://standards.freedesktop.org/basedir-spec/basedir-spec-latest.html
type xdgDirs struct {
	configHome string
	configDirs []string
}

// newXDGDirs reads directory locations from the given environment variables.
func newXDGDirs(environ []string) *xdgDirs {
	x := &xdgDirs{
		configHome: getenv(environ, "XDG_CONFIG_HOME"),
		configDirs: filepath.SplitList(getenv(environ, "XDG_CONFIG_DIRS")),
	}
	if x.configHome == "" {
		if home := getenv(environ, "HOME"); home != "" {
			x.configHome = filepath.Join(home, ".config")
		}
	}
	if len(x.configDirs) == 0 {
		x.configDirs = []string{"/etc/xdg"}
	}
	return x
}

// readConfig reads the file at the given slash-separated path relative
// to the gg config directory.
func (x *xdgDirs) readConfig(name string) ([]byte, error) {
	relpath := filepath.Join("gg", filepath.FromSlash(name))
	for _, dir := range x.configPaths() {
		data, err := ioutil.ReadFile(filepath.Join(dir, relpath))
		if err == nil {
			return data, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return nil, &os.PathError{
		Op:   "open",
		Path: filepath.Join("$XDG_CONFIG_HOME", relpath),
		Err:  os.ErrNotExist,
	}
}

// configPaths returns the list of directories to search for
// configuration files in descending order of precedence. The caller
// must not modify the returned slice.
func (x *xdgDirs) configPaths() []string {
	if x.configHome == "" {
		return x.configDirs
	}
	return append([]string{x.configHome}, x.configDirs...)
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
