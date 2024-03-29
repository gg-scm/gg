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

// gg is Git with less typing.
//
// Learn more at https://gg-scm.io/
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"

	"gg-scm.io/pkg/git"
	"gg-scm.io/pkg/git/packfile/client"
	"gg-scm.io/tool/internal/flag"
	"gg-scm.io/tool/internal/repocache"
	"gg-scm.io/tool/internal/sigterm"
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
	const description = "Git with less typing\n\n" +
		"basic commands:\n" +
		"  add           " + addSynopsis + "\n" +
		"  addremove     " + addRemoveSynopsis + "\n" +
		"  branch        " + branchSynopsis + "\n" +
		"  cat           " + catSynopsis + "\n" +
		"  clone         " + cloneSynopsis + "\n" +
		"  commit        " + commitSynopsis + "\n" +
		"  diff          " + diffSynopsis + "\n" +
		"  identify      " + identifySynopsis + "\n" +
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
		"  github-login  " + gitHubLoginSynopsis + "\n" +
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
			return fmt.Errorf("gg: %w", err)
		}
	}
	opts := git.Options{
		GitExe: *gitPath,
		Dir:    pctx.dir,
		Env:    pctx.env,
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
	git, err := git.New(opts)
	if err != nil {
		return fmt.Errorf("gg: %w", err)
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
		if err := showVersion(ctx, cc); err != nil {
			return fmt.Errorf("gg: %w", err)
		}
		return nil
	}
	err = dispatch(ctx, cc, globalFlags, globalFlags.Arg(0), globalFlags.Args()[1:])
	if err != nil {
		return fmt.Errorf("gg: %w", err)
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

func (cc *cmdContext) interactiveGit(ctx context.Context, args ...string) error {
	err := cc.git.Runner().RunGit(ctx, &git.Invocation{
		Dir:    cc.dir,
		Args:   args,
		Stdin:  cc.stdin,
		Stdout: cc.stdout,
		Stderr: cc.stderr,
	})
	if err != nil {
		return fmt.Errorf("git %s: %w", args[0], err)
	}
	return nil
}

func dispatch(ctx context.Context, cc *cmdContext, globalFlags *flag.FlagSet, name string, args []string) error {
	switch name {
	case "add":
		return add(ctx, cc, args)
	case "addremove":
		return addRemove(ctx, cc, args)
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
	case "github-login":
		return gitHubLogin(ctx, cc, args)
	case "histedit":
		return histedit(ctx, cc, args)
	case "identify", "id":
		return identify(ctx, cc, args)
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

const repoCacheFileName = "gg-cache.db"

func openRepoCache(ctx context.Context, commonDir string, sync bool) (*repocache.Cache, error) {
	cache, err := repocache.Open(ctx, filepath.Join(commonDir, repoCacheFileName))
	if err != nil {
		return nil, err
	}
	if !sync {
		return cache, nil
	}
	remote, err := client.NewRemote(client.URLFromPath(commonDir), &client.Options{
		UserAgent: userAgentString(),
	})
	if err != nil {
		cache.Close()
		return nil, fmt.Errorf("open repository cache for %s: %v", commonDir, err)
	}
	if err := cache.CopyFrom(ctx, remote); err != nil {
		cache.Close()
		return nil, fmt.Errorf("open repository cache for %s: %v", commonDir, err)
	}
	return cache, nil
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
	commit, localMods := strings.CutSuffix(buildCommit, "+")
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
	out, err := cc.git.Output(ctx, "--version")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(cc.stdout, out); err != nil {
		return err
	}
	return nil
}

func userAgentString() string {
	if versionInfo == "" {
		return "gg-scm.io"
	}
	return "gg-scm.io " + versionInfo
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
	cacheHome  string
}

// newXDGDirs reads directory locations from the given environment variables.
func newXDGDirs(environ []string) *xdgDirs {
	x := &xdgDirs{
		configHome: getenv(environ, "XDG_CONFIG_HOME"),
		configDirs: filepath.SplitList(getenv(environ, "XDG_CONFIG_DIRS")),
		cacheHome:  getenv(environ, "XDG_CACHE_HOME"),
	}
	if x.configHome == "" {
		if home := getenv(environ, "HOME"); home != "" {
			x.configHome = filepath.Join(home, ".config")
		}
	}
	if len(x.configDirs) == 0 {
		x.configDirs = []string{"/etc/xdg"}
	}
	if x.cacheHome == "" {
		if home := getenv(environ, "HOME"); home != "" {
			x.cacheHome = filepath.Join(home, ".cache")
		}
	}
	return x
}

// configDirname is the name of the subdirectory inside the user's configuration
// or cache directory to store files.
const configDirname = "gg"

// readConfig reads the file at the given slash-separated path relative
// to the gg config directory.
func (x *xdgDirs) readConfig(name string) ([]byte, error) {
	relpath := filepath.Join(configDirname, filepath.FromSlash(name))
	for _, dir := range x.configPaths() {
		data, err := os.ReadFile(filepath.Join(dir, relpath))
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

// writeSecret writes the file at the given slash-separated path relative to the
// gg directory with restricted permissions.
func (x *xdgDirs) writeSecret(name string, value []byte) error {
	path := filepath.Join(x.configHome, configDirname, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, value, 0600); err != nil {
		return err
	}
	return nil
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

// openCache opens the file at the given slash-separated path relative
// to the gg cache directory for reading. It is the caller's
// responsibility to close the file.
func (x *xdgDirs) openCache(name string) (*os.File, error) {
	if x.cacheHome == "" {
		return nil, fmt.Errorf("open cache %s: no $XDG_CACHE_HOME variable set", name)
	}
	f, err := os.Open(filepath.Join(x.cacheHome, configDirname, filepath.FromSlash(name)))
	if err != nil {
		return nil, fmt.Errorf("open cache %s: %w", name, err)
	}
	return f, nil
}

// createCache opens the file at the given slash-separated path relative
// to the gg cache directory for writing. Any non-existent parent
// directories will be created. It is the caller's responsibility to
// close the file.
func (x *xdgDirs) createCache(name string) (*os.File, error) {
	if x.cacheHome == "" {
		return nil, fmt.Errorf("create cache %s: no $XDG_CACHE_HOME variable set", name)
	}
	relpath := filepath.Join(x.cacheHome, configDirname, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(relpath), 0755); err != nil {
		return nil, fmt.Errorf("create cache %s: %w", name, err)
	}
	f, err := os.Create(relpath)
	if err != nil {
		return nil, fmt.Errorf("create cache %s: %w", name, err)
	}
	return f, nil
}

type usageError string

func usagef(format string, args ...interface{}) error {
	e := usageError(fmt.Sprintf(format, args...))
	return &e
}

func (ue *usageError) Error() string {
	return "usage: " + string(*ue)
}

func isUsage(e error) bool {
	return errors.As(e, new(*usageError))
}
