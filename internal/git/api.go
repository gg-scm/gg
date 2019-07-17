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

package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gg-scm.io/pkg/internal/sigterm"
	"golang.org/x/xerrors"
)

// WorkTree determines the absolute path of the root of the current
// working tree given the configuration. Any symlinks are resolved.
func (g *Git) WorkTree(ctx context.Context) (string, error) {
	const errPrefix = "find git work tree root"
	out, err := g.output(ctx, errPrefix, []string{g.exe, "rev-parse", "--show-toplevel"})
	if err != nil {
		return "", err
	}
	line, err := oneLine(out)
	if err != nil {
		return "", xerrors.Errorf("%s: %w", errPrefix, err)
	}
	evaled, err := filepath.EvalSymlinks(line)
	if err != nil {
		return "", xerrors.Errorf("%s: %w", errPrefix, err)
	}
	return evaled, nil
}

// prefix returns the path of the working directory relative to the root
// of the working tree.
func (g *Git) prefix(ctx context.Context) (string, error) {
	const errPrefix = "prefix"
	prefix, err := g.output(ctx, errPrefix, []string{g.exe, "rev-parse", "--show-prefix"})
	if err != nil {
		return "", err
	}
	prefix, err = oneLine(prefix)
	if err != nil {
		return "", xerrors.Errorf("%s: %w", errPrefix, err)
	}
	return prefix, nil
}

// GitDir determines the absolute path of the Git directory for this
// working tree given the configuration. Any symlinks are resolved.
func (g *Git) GitDir(ctx context.Context) (string, error) {
	// TODO(someday): Use --absolute-git-dir when minimum supported
	// Git version >= 2.13.2.
	const errPrefix = "find .git directory"
	out, err := g.output(ctx, errPrefix, []string{g.exe, "rev-parse", "--git-common-dir"})
	if err != nil {
		return "", err
	}
	line, err := oneLine(out)
	if err != nil {
		return "", xerrors.Errorf("%s: %w", errPrefix, err)
	}
	var path string
	if filepath.IsAbs(line) {
		path = filepath.Clean(line)
	} else {
		path = filepath.Join(g.dir, line)
	}
	evaled, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", xerrors.Errorf("%s: %w", errPrefix, err)
	}
	return evaled, nil
}

// CommonDir determines the absolute path of the Git directory, possibly
// shared among different working trees, given the configuration. Any
// symlinks are resolved.
func (g *Git) CommonDir(ctx context.Context) (string, error) {
	const errPrefix = "find .git directory"
	out, err := g.output(ctx, errPrefix, []string{g.exe, "rev-parse", "--git-common-dir"})
	if err != nil {
		return "", err
	}
	line, err := oneLine(out)
	if err != nil {
		return "", xerrors.Errorf("%s: %w", errPrefix, err)
	}
	var path string
	if filepath.IsAbs(line) {
		path = filepath.Clean(line)
	} else {
		path = filepath.Join(g.dir, line)
	}
	evaled, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", xerrors.Errorf("%s: %w", errPrefix, err)
	}
	return evaled, nil
}

// IsMerging reports whether the index has a pending merge commit.
func (g *Git) IsMerging(ctx context.Context) (bool, error) {
	const errPrefix = "check git merge"
	gitDir, err := g.GitDir(ctx)
	if err != nil {
		return false, xerrors.Errorf("%s: %w", errPrefix, err)
	}
	_, err = os.Stat(filepath.Join(gitDir, "MERGE_HEAD"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, xerrors.Errorf("%s: %w", errPrefix, err)
	}
	return true, nil
}

// MergeBase returns the best common ancestor between two commits to use
// in a three-way merge.
func (g *Git) MergeBase(ctx context.Context, rev1, rev2 string) (Hash, error) {
	errPrefix := fmt.Sprintf("git merge-base %q %q", rev1, rev2)
	if err := validateRev(rev1); err != nil {
		return Hash{}, xerrors.Errorf("%s: %w", errPrefix, err)
	}
	if err := validateRev(rev2); err != nil {
		return Hash{}, xerrors.Errorf("%s: %w", errPrefix, err)
	}
	out, err := g.output(ctx, errPrefix, []string{g.exe, "merge-base", rev1, rev2})
	if err != nil {
		return Hash{}, err
	}
	h, err := ParseHash(strings.TrimSuffix(out, "\n"))
	if err != nil {
		return Hash{}, xerrors.Errorf("%s: %w", errPrefix, err)
	}
	return h, nil
}

// IsAncestor reports whether rev1 is an ancestor of rev2.
// If rev1 == rev2, then IsAncestor returns true.
func (g *Git) IsAncestor(ctx context.Context, rev1, rev2 string) (bool, error) {
	errPrefix := fmt.Sprintf("git: check %q ancestor of %q", rev1, rev2)
	if err := validateRev(rev1); err != nil {
		return false, xerrors.Errorf("%s: %w", errPrefix, err)
	}
	if err := validateRev(rev2); err != nil {
		return false, xerrors.Errorf("%s: %w", errPrefix, err)
	}
	c := g.command(ctx, []string{g.exe, "merge-base", "--is-ancestor", rev1, rev2})
	stderr := new(bytes.Buffer)
	c.Stderr = stderr
	if err := sigterm.Run(ctx, c); err != nil {
		if err, ok := err.(*exec.ExitError); ok && exitStatus(err.ProcessState) == 1 {
			return false, nil
		}
		return false, commandError(errPrefix, err, stderr.Bytes())
	}
	return true, nil
}

// ListTree returns the list of files at a given revision.
// If pathspecs is not empty, then it is used to filter the paths.
func (g *Git) ListTree(ctx context.Context, rev string, pathspecs []Pathspec) (map[TopPath]struct{}, error) {
	errPrefix := fmt.Sprintf("git ls-tree %q", rev)
	if err := validateRev(rev); err != nil {
		return nil, xerrors.Errorf("%s: %w", errPrefix, err)
	}
	args := []string{g.exe, "ls-tree", "-z", "-r", "--name-only"}
	if len(pathspecs) == 0 {
		args = append(args, "--full-tree", rev)
	} else {
		// Use --full-name, as --full-tree interprets the path arguments
		// relative to the top of the directory.
		args = append(args, "--full-name", rev, "--")
		for _, p := range pathspecs {
			args = append(args, p.String())
		}
	}
	out, err := g.output(ctx, errPrefix, args)
	if err != nil {
		return nil, err
	}
	paths := make(map[TopPath]struct{})
	for len(out) > 0 {
		i := strings.IndexByte(out, 0)
		if i == -1 {
			return paths, xerrors.Errorf("%s: %w", errPrefix, io.ErrUnexpectedEOF)
		}
		paths[TopPath(out[:i])] = struct{}{}
		out = out[i+1:]
	}
	return paths, nil
}

// Cat reads the content of a file at a particular revision.
// It is the caller's responsibility to close the returned io.ReadCloser
// if the returned error is nil.
func (g *Git) Cat(ctx context.Context, rev string, path TopPath) (io.ReadCloser, error) {
	errPrefix := fmt.Sprintf("git cat %q @ %q", path, rev)
	if err := validateRev(rev); err != nil {
		return nil, xerrors.Errorf("%s: %w", errPrefix, err)
	}
	if strings.Contains(rev, ":") {
		return nil, xerrors.Errorf("%s: revision contains ':'", errPrefix)
	}
	if path == "" {
		return nil, xerrors.Errorf("%s: empty path", errPrefix)
	}
	if strings.HasPrefix(string(path), "./") || strings.HasPrefix(string(path), "../") {
		return nil, xerrors.Errorf("%s: path is relative", errPrefix)
	}
	c := g.command(ctx, []string{g.exe, "cat-file", "blob", rev + ":" + path.String()})
	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, xerrors.Errorf("%s: %w", errPrefix, err)
	}
	stderr := new(bytes.Buffer)
	c.Stderr = &limitWriter{w: stderr, n: 4096}
	wait, err := sigterm.Start(ctx, c)
	if err != nil {
		stdout.Close()
		return nil, xerrors.Errorf("%s: %w", errPrefix, err)
	}

	// If Git reports an error, stdout will be empty and stderr will
	// contain the error message.
	first := make([]byte, 2048)
	readLen, readErr := io.ReadAtLeast(stdout, first, 1)
	if readErr != nil {
		// Empty stdout, check for error.
		if err := wait(); err != nil {
			return nil, commandError(errPrefix, err, stderr.Bytes())
		}
		if readErr != io.EOF {
			return nil, commandError(errPrefix, readErr, stderr.Bytes())
		}
		return nopReader{}, nil
	}
	return &catReader{
		errPrefix: errPrefix,
		first:     first[:readLen],
		pipe:      stdout,
		wait:      wait,
		stderr:    stderr,
	}, nil
}

type catReader struct {
	errPrefix string
	first     []byte
	pipe      io.ReadCloser
	wait      func() error
	stderr    *bytes.Buffer // can't be read until wait returns
}

func (cr *catReader) Read(p []byte) (int, error) {
	if len(cr.first) > 0 {
		n := copy(p, cr.first)
		cr.first = cr.first[n:]
		return n, nil
	}
	return cr.pipe.Read(p)
}

func (cr *catReader) Close() error {
	closeErr := cr.pipe.Close()
	waitErr := cr.wait()
	if waitErr != nil {
		return commandError("close "+cr.errPrefix, waitErr, cr.stderr.Bytes())
	}
	if closeErr != nil {
		return xerrors.Errorf("close %s: %w", cr.errPrefix, closeErr)
	}
	return nil
}

// Init creates a new empty repository at the given path. Any relative
// paths are interpreted relative to the Git process's working
// directory. If any of the repository's parent directories don't exist,
// they will be created.
func (g *Git) Init(ctx context.Context, dir string) error {
	c := g.command(ctx, []string{g.exe, "init", "--quiet", "--", dir})
	buf := new(bytes.Buffer)
	c.Stdout = &limitWriter{w: buf, n: 4096}
	c.Stderr = c.Stdout
	if err := sigterm.Run(ctx, c); err != nil {
		return commandError(fmt.Sprintf("git init %q", dir), err, buf.Bytes())
	}
	return nil
}

// InitBare creates a new empty, bare repository at the given path. Any
// relative paths are interpreted relative to the Git process's working
// directory. If any of the repository's parent directories don't exist,
// they will be created.
func (g *Git) InitBare(ctx context.Context, dir string) error {
	c := g.command(ctx, []string{g.exe, "init", "--quiet", "--bare", "--", dir})
	buf := new(bytes.Buffer)
	c.Stdout = &limitWriter{w: buf, n: 4096}
	c.Stderr = c.Stdout
	if err := sigterm.Run(ctx, c); err != nil {
		return commandError(fmt.Sprintf("git init %q", dir), err, buf.Bytes())
	}
	return nil
}

// AddOptions specifies the command-line options for `git add`.
type AddOptions struct {
	// IncludeIgnored specifies whether to add ignored files.
	// If this is false and an ignored file is explicitly named, then Add
	// will return an error while other matched files are still added.
	IncludeIgnored bool
	// If IntentToAdd is true, then contents of files in the index will
	// not be changed, but any untracked files will have entries added
	// into the index with empty content.
	IntentToAdd bool
}

// Add adds file contents to the index.
func (g *Git) Add(ctx context.Context, pathspecs []Pathspec, opts AddOptions) error {
	args := []string{g.exe, "add"}
	if opts.IncludeIgnored {
		args = append(args, "-f")
	}
	if opts.IntentToAdd {
		args = append(args, "-N")
	}
	args = append(args, "--")
	for _, p := range pathspecs {
		args = append(args, p.String())
	}
	c := g.command(ctx, args)
	buf := new(bytes.Buffer)
	c.Stdout = &limitWriter{w: buf, n: 4096}
	c.Stderr = c.Stdout
	if err := sigterm.Run(ctx, c); err != nil {
		return commandError("git add", err, buf.Bytes())
	}
	return nil
}

// StageTracked updates the index to match the tracked files in the
// working copy.
func (g *Git) StageTracked(ctx context.Context) error {
	return g.run(ctx, "git add -A", []string{g.exe, "add", "--all"})
}

// RemoveOptions specifies the command-line options for `git add`.
type RemoveOptions struct {
	// Recursive specifies whether to remove directories.
	Recursive bool
	// If Modified is true, then files will be deleted even if they've
	// been modified from their checked in state.
	Modified bool
	// If KeepWorkingCopy is true, then the file will only be removed in
	// the index, not the working copy.
	KeepWorkingCopy bool
}

// Remove removes file contents from the index.
func (g *Git) Remove(ctx context.Context, pathspecs []Pathspec, opts RemoveOptions) error {
	if len(pathspecs) == 0 {
		return nil
	}
	args := []string{g.exe, "rm", "--quiet"}
	if opts.Recursive {
		args = append(args, "-r")
	}
	if opts.Modified {
		args = append(args, "--force")
	}
	if opts.KeepWorkingCopy {
		args = append(args, "--cached")
	}
	args = append(args, "--")
	for _, p := range pathspecs {
		args = append(args, p.String())
	}
	c := g.command(ctx, args)
	buf := new(bytes.Buffer)
	c.Stdout = &limitWriter{w: buf, n: 4096}
	c.Stderr = c.Stdout
	if err := sigterm.Run(ctx, c); err != nil {
		return commandError("git rm", err, buf.Bytes())
	}
	return nil
}

// CommitOptions overrides the default metadata for a commit. Any fields
// with zero values will use the value inferred from Git's environment.
type CommitOptions struct {
	Author     User
	AuthorTime time.Time
	Committer  User
	CommitTime time.Time
}

func (opts CommitOptions) addToEnv(env []string) []string {
	if opts.Author.Name != "" {
		env = append(env, "GIT_AUTHOR_NAME="+opts.Author.Name)
	}
	if opts.Author.Email != "" {
		env = append(env, "GIT_AUTHOR_EMAIL="+opts.Author.Email)
	}
	if !opts.AuthorTime.IsZero() {
		env = append(env, "GIT_AUTHOR_DATE="+opts.AuthorTime.Format(time.RFC3339))
	}
	if opts.Committer.Name != "" {
		env = append(env, "GIT_COMMITTER_NAME="+opts.Committer.Name)
	}
	if opts.Committer.Email != "" {
		env = append(env, "GIT_COMMITTER_EMAIL="+opts.Committer.Email)
	}
	if !opts.CommitTime.IsZero() {
		env = append(env, "GIT_COMMITTER_DATE="+opts.CommitTime.Format(time.RFC3339))
	}
	return env
}

// Commit creates a new commit on HEAD with the staged content.
// The message will be used exactly as given.
func (g *Git) Commit(ctx context.Context, message string, opts CommitOptions) error {
	c := g.command(ctx, []string{g.exe, "commit", "--quiet", "--file=-", "--cleanup=verbatim"})
	c.Env = opts.addToEnv(c.Env) // c.Env == g.env; len(g.env) == cap(g.env)
	c.Stdin = strings.NewReader(message)
	out := new(bytes.Buffer)
	c.Stdout = &limitWriter{w: out, n: 4096}
	c.Stderr = c.Stdout
	if err := sigterm.Run(ctx, c); err != nil {
		return commandError("git commit", err, out.Bytes())
	}
	return nil
}

// CommitAll creates a new commit on HEAD with all of the tracked files.
// The message will be used exactly as given.
func (g *Git) CommitAll(ctx context.Context, message string, opts CommitOptions) error {
	c := g.command(ctx, []string{g.exe, "commit", "--quiet", "--file=-", "--cleanup=verbatim", "--all"})
	c.Env = opts.addToEnv(c.Env) // c.Env == g.env; len(g.env) == cap(g.env)
	c.Stdin = strings.NewReader(message)
	out := new(bytes.Buffer)
	c.Stdout = &limitWriter{w: out, n: 4096}
	c.Stderr = c.Stdout
	if err := sigterm.Run(ctx, c); err != nil {
		return commandError("git commit", err, out.Bytes())
	}
	return nil
}

// CommitFiles creates a new commit on HEAD that updates the given files
// to the content in the working copy. The message will be used exactly
// as given.
func (g *Git) CommitFiles(ctx context.Context, message string, pathspecs []Pathspec, opts CommitOptions) error {
	const errPrefix = "git commit"
	if len(pathspecs) > 0 {
		prefix, err := g.prefix(ctx)
		if err != nil {
			return xerrors.Errorf("%s: %w", errPrefix, err)
		}
		if prefix != "" {
			// Always run from top of worktree to avoid Git bug detailed in
			// https://github.com/zombiezen/gg/issues/10
			workTree, err := g.WorkTree(ctx)
			if err != nil {
				return xerrors.Errorf("%s: %w", errPrefix, err)
			}
			g = g.WithDir(workTree)

			// Rewrite pathspecs as needed.
			pathspecs = append([]Pathspec(nil), pathspecs...)
			for i := range pathspecs {
				magic, pat := pathspecs[i].SplitMagic()
				if magic.Top || filepath.IsAbs(pat) {
					// Top-level or absolute paths need no rewrite.
					continue
				}
				pathspecs[i] = JoinPathspecMagic(magic, filepath.Join(prefix, pat))
			}
		}
	}
	args := []string{g.exe, "commit", "--quiet", "--file=-", "--cleanup=verbatim", "--only", "--allow-empty", "--"}
	for _, spec := range pathspecs {
		args = append(args, spec.String())
	}
	c := g.command(ctx, args)
	c.Env = opts.addToEnv(c.Env) // c.Env == g.env; len(g.env) == cap(g.env)
	c.Stdin = strings.NewReader(message)
	out := new(bytes.Buffer)
	c.Stdout = &limitWriter{w: out, n: 4096}
	c.Stderr = c.Stdout
	if err := sigterm.Run(ctx, c); err != nil {
		return commandError(errPrefix, err, out.Bytes())
	}
	return nil
}

// AmendOptions overrides the previous commit's fields.
type AmendOptions struct {
	// If Message is not empty, it is the commit message that will be used.
	// Otherwise, the previous commit's message will be used.
	Message string
	// If Author is filled out, then it will be used as the commit author.
	// If Author is the zero value, then the previous commit's author will
	// be used. If Author is partially filled out, the Amend methods will
	// return an error.
	Author User
	// If AuthorTime is not zero, then it will be used as the author time.
	// Otherwise, the previous commit's author time will be used.
	AuthorTime time.Time

	// Committer fields set to non-zero values will override the default
	// committer information from Git configuration.
	Committer User
	// If CommitTime is not zero, then it will be used as the commit time
	// instead of now.
	CommitTime time.Time
}

func (opts AmendOptions) validate() error {
	if (opts.Author.Name != "") != (opts.Author.Email != "") {
		return xerrors.New("author partially filled")
	}
	return nil
}

func (opts AmendOptions) addAuthorToArgs(args []string) []string {
	if opts.Author != (User{}) {
		args = append(args, "--author="+opts.Author.String())
	}
	if !opts.AuthorTime.IsZero() {
		args = append(args, "--date="+opts.AuthorTime.Format(time.RFC3339))
	}
	return args
}

func (opts AmendOptions) addToEnv(env []string) []string {
	if opts.Committer.Name != "" {
		env = append(env, "GIT_COMMITTER_NAME="+opts.Committer.Name)
	}
	if opts.Committer.Email != "" {
		env = append(env, "GIT_COMMITTER_EMAIL="+opts.Committer.Email)
	}
	if !opts.CommitTime.IsZero() {
		env = append(env, "GIT_COMMITTER_DATE="+opts.CommitTime.Format(time.RFC3339))
	}
	return env
}

// Amend replaces the tip of the current branch with a new commit with
// the content of the index.
func (g *Git) Amend(ctx context.Context, opts AmendOptions) error {
	const errPrefix = "git commit --amend"
	if err := opts.validate(); err != nil {
		return xerrors.Errorf("%s: %w", errPrefix, err)
	}
	msg := opts.Message
	if msg == "" {
		info, err := g.CommitInfo(ctx, "HEAD")
		if err != nil {
			return xerrors.Errorf("%s: %w", errPrefix, err)
		}
		msg = info.Message
	}
	c := g.command(ctx, opts.addAuthorToArgs([]string{
		g.exe,
		"commit",
		"--amend",
		"--quiet",
		"--file=-",
		"--cleanup=verbatim",
	}))
	c.Env = opts.addToEnv(c.Env) // c.Env == g.env; len(g.env) == cap(g.env)
	c.Stdin = strings.NewReader(msg)
	out := new(bytes.Buffer)
	c.Stdout = &limitWriter{w: out, n: 4096}
	c.Stderr = c.Stdout
	if err := sigterm.Run(ctx, c); err != nil {
		return commandError(errPrefix, err, out.Bytes())
	}
	return nil
}

// AmendAll replaces the tip of the current branch with a new commit
// with the content of the working copy for all tracked files.
func (g *Git) AmendAll(ctx context.Context, opts AmendOptions) error {
	const errPrefix = "git commit --amend --all"
	if err := opts.validate(); err != nil {
		return xerrors.Errorf("%s: %w", errPrefix, err)
	}
	msg := opts.Message
	if msg == "" {
		info, err := g.CommitInfo(ctx, "HEAD")
		if err != nil {
			return xerrors.Errorf("%s: %w", errPrefix, err)
		}
		msg = info.Message
	}
	c := g.command(ctx, opts.addAuthorToArgs([]string{
		g.exe,
		"commit",
		"--amend",
		"--all",
		"--quiet",
		"--file=-",
		"--cleanup=verbatim",
	}))
	c.Env = opts.addToEnv(c.Env) // c.Env == g.env; len(g.env) == cap(g.env)
	c.Stdin = strings.NewReader(msg)
	out := new(bytes.Buffer)
	c.Stdout = &limitWriter{w: out, n: 4096}
	c.Stderr = c.Stdout
	if err := sigterm.Run(ctx, c); err != nil {
		return commandError(errPrefix, err, out.Bytes())
	}
	return nil
}

// AmendFiles replaces the tip of the current branch with a new commit
// with the content of the named files from the working copy. Files not
// named will get their content from the previous commit.
//
// Notably, AmendFiles with no paths will not change the file content of
// the commit, just the options specified.
func (g *Git) AmendFiles(ctx context.Context, pathspecs []Pathspec, opts AmendOptions) error {
	const errPrefix = "git commit --amend --only"
	if err := opts.validate(); err != nil {
		return xerrors.Errorf("%s: %w", errPrefix, err)
	}
	msg := opts.Message
	if msg == "" {
		info, err := g.CommitInfo(ctx, "HEAD")
		if err != nil {
			return xerrors.Errorf("%s: %w", errPrefix, err)
		}
		msg = info.Message
	}
	if len(pathspecs) > 0 {
		prefix, err := g.prefix(ctx)
		if err != nil {
			return xerrors.Errorf("%s: %w", errPrefix, err)
		}
		if prefix != "" {
			// Always run from top of worktree to avoid Git bug detailed in
			// https://github.com/zombiezen/gg/issues/10
			workTree, err := g.WorkTree(ctx)
			if err != nil {
				return xerrors.Errorf("%s: %w", errPrefix, err)
			}
			g = g.WithDir(workTree)

			// Rewrite pathspecs as needed.
			pathspecs = append([]Pathspec(nil), pathspecs...)
			for i := range pathspecs {
				magic, pat := pathspecs[i].SplitMagic()
				if magic.Top || filepath.IsAbs(pat) {
					// Top-level or absolute paths need no rewrite.
					continue
				}
				pathspecs[i] = JoinPathspecMagic(magic, filepath.Join(prefix, pat))
			}
		}
	}
	args := opts.addAuthorToArgs([]string{
		g.exe,
		"commit",
		"--amend",
		"--only",
		"--quiet",
		"--file=-",
		"--cleanup=verbatim",
	})
	if len(pathspecs) > 0 {
		args = append(args, "--")
		for _, p := range pathspecs {
			args = append(args, p.String())
		}
	}
	c := g.command(ctx, args)
	c.Env = opts.addToEnv(c.Env) // c.Env == g.env; len(g.env) == cap(g.env)
	c.Stdin = strings.NewReader(msg)
	out := new(bytes.Buffer)
	c.Stdout = &limitWriter{w: out, n: 4096}
	c.Stderr = c.Stdout
	if err := sigterm.Run(ctx, c); err != nil {
		return commandError(errPrefix, err, out.Bytes())
	}
	return nil
}

// Merge merges changes from the named revisions into the index and the
// working copy. It updates MERGE_HEAD but does not create a commit.
// Merge will never perform a fast-forward merge.
//
// In case of conflict, Merge will return an error but still update
// MERGE_HEAD. To check for this condition, call IsMerging after
// receiving an error from Merge (verifying that IsMerging returned
// false before calling Merge).
func (g *Git) Merge(ctx context.Context, revs []string) error {
	errPrefix := "git merge"
	if len(revs) == 0 {
		return xerrors.New(errPrefix + ": no revisions")
	}
	for _, rev := range revs {
		if err := validateRev(rev); err != nil {
			return xerrors.Errorf("%s: %w", errPrefix, err)
		}
	}
	if len(revs) == 1 {
		errPrefix += " " + revs[0]
	}
	args := []string{g.exe, "merge", "--quiet", "--no-commit", "--no-ff"}
	args = append(args, revs...)
	return g.run(ctx, errPrefix, args)
}

// AbortMerge aborts the current conflict resolution process and tries
// to reconstruct pre-merge state.
func (g *Git) AbortMerge(ctx context.Context) error {
	return g.run(ctx, "git abort merge", []string{g.exe, "merge", "--abort"})
}

// CheckoutOptions specifies the command-line options for `git checkout`.
type CheckoutOptions struct {
	// ConflictBehavior specifies the behavior when encountering locally
	// modified files.
	ConflictBehavior CheckoutConflictBehavior
}

// CheckoutConflictBehavior specifies the behavior of checkout with
// local modifications.
type CheckoutConflictBehavior int

// Possible checkout behaviors when encountering locally modified files.
const (
	// AbortOnFileChange stops the checkout if a file that is modified
	// locally differs between the current HEAD and the target commit.
	// This is the default behavior.
	AbortOnFileChange CheckoutConflictBehavior = iota
	// MergeLocal performs a three-way merge on any differing files.
	MergeLocal
	// DiscardLocal uses the target commit's content regardless of local
	// modifications.
	DiscardLocal
)

// String returns the Go constant name of the behavior.
func (ccb CheckoutConflictBehavior) String() string {
	switch ccb {
	case AbortOnFileChange:
		return "AbortOnFileChange"
	case MergeLocal:
		return "MergeLocal"
	case DiscardLocal:
		return "DiscardLocal"
	default:
		return fmt.Sprintf("CheckoutConflictBehavior(%d)", int(ccb))
	}
}

// CheckoutBranch switches HEAD to another branch and updates the
// working copy to match. If the branch does not exist, then
// CheckoutBranch returns an error.
func (g *Git) CheckoutBranch(ctx context.Context, branch string, opts CheckoutOptions) error {
	errPrefix := fmt.Sprintf("git checkout %q", branch)
	if err := validateBranch(branch); err != nil {
		return xerrors.Errorf("%s: %w", errPrefix, err)
	}
	if opts.ConflictBehavior != AbortOnFileChange && opts.ConflictBehavior != MergeLocal && opts.ConflictBehavior != DiscardLocal {
		return xerrors.Errorf("%s: unknown conflict behavior in options", errPrefix)
	}
	// Verify that the branch exists. `git checkout` will attempt to
	// create branches if they don't exist if there's a remote tracking
	// branch of the same name.
	if err := g.run(ctx, errPrefix, []string{g.exe, "rev-parse", "-q", "--verify", "--revs-only", BranchRef(branch).String()}); err != nil {
		return err
	}

	// Run checkout with branch name.
	args := []string{g.exe, "checkout", "--quiet"}
	switch opts.ConflictBehavior {
	case MergeLocal:
		args = append(args, "--merge")
	case DiscardLocal:
		args = append(args, "--force")
	}
	args = append(args, branch, "--")
	if err := g.run(ctx, errPrefix, args); err != nil {
		return err
	}
	return nil
}

// CheckoutRev switches HEAD to a specific commit and updates the
// working copy to match. It will always put the worktree in "detached
// HEAD" state.
func (g *Git) CheckoutRev(ctx context.Context, rev string, opts CheckoutOptions) error {
	errPrefix := fmt.Sprintf("git checkout --detach %q", rev)
	if err := validateRev(rev); err != nil {
		return xerrors.Errorf("%s: %w", errPrefix, err)
	}

	// Run checkout with the revision.
	args := []string{g.exe, "checkout", "--quiet", "--detach"}
	switch opts.ConflictBehavior {
	case MergeLocal:
		args = append(args, "--merge")
	case DiscardLocal:
		args = append(args, "--force")
	}
	args = append(args, rev, "--")
	if err := g.run(ctx, errPrefix, args); err != nil {
		return err
	}
	return nil
}

// BranchOptions specifies options for a new branch.
type BranchOptions struct {
	// StartPoint is a revision to start from. If empty, then HEAD is used.
	StartPoint string
	// If Checkout is true, then HEAD and the working copy will be
	// switched to the new branch.
	Checkout bool
	// If Overwrite is true and a branch with the given name already
	// exists, then it will be reset to the start point. No other branch
	// information is modified, like the upstream.
	Overwrite bool
	// If Track is true and StartPoint names a ref, then the upstream of
	// the branch will be set to the ref named by StartPoint.
	Track bool
}

// NewBranch creates a new branch, a ref of the form "refs/heads/NAME",
// where NAME is the name argument.
func (g *Git) NewBranch(ctx context.Context, name string, opts BranchOptions) error {
	errPrefix := fmt.Sprintf("git branch %q", name)
	if err := validateBranch(name); err != nil {
		return xerrors.Errorf("%s: %w", errPrefix, err)
	}
	if opts.StartPoint != "" {
		if err := validateRev(opts.StartPoint); err != nil {
			return xerrors.Errorf("%s: %w", errPrefix, err)
		}
	}
	args := []string{g.exe}
	if opts.Checkout {
		args = append(args, "checkout", "--quiet")
		if opts.Track {
			args = append(args, "--track")
		} else {
			args = append(args, "--no-track")
		}
		if opts.Overwrite {
			args = append(args, "-B", name)
		} else {
			args = append(args, "-b", name)
		}
		if opts.StartPoint != "" {
			args = append(args, opts.StartPoint, "--")
		}
	} else {
		args = append(args, "branch", "--quiet")
		if opts.Track {
			args = append(args, "--track")
		} else {
			args = append(args, "--no-track")
		}
		if opts.Overwrite {
			args = append(args, "--force")
		}
		args = append(args, name)
		if opts.StartPoint != "" {
			args = append(args, opts.StartPoint)
		}
	}
	return g.run(ctx, errPrefix, args)
}

// commandError returns a new error with the information from an
// unsuccessful run of a subprocess.
func commandError(prefix string, runError error, stderr []byte) error {
	stderr = bytes.TrimSuffix(stderr, []byte{'\n'})
	if len(stderr) == 0 {
		return xerrors.Errorf("%s: %w", prefix, runError)
	}
	if _, isExit := runError.(*exec.ExitError); isExit {
		if bytes.IndexByte(stderr, '\n') == -1 {
			// Collapse into single line.
			return xerrors.Errorf("%s: %s", prefix, stderr)
		}
		return xerrors.Errorf("%s:\n%s", prefix, stderr)
	}
	// TODO(someday): Use %w when https://golang.org/issue/33143 is fixed.
	return xerrors.Errorf("%s: %v\n%s", prefix, runError, stderr)
}

func validateRev(rev string) error {
	if rev == "" {
		return xerrors.New("empty revision")
	}
	if strings.HasPrefix(rev, "-") {
		return xerrors.New("revision cannot begin with dash")
	}
	return nil
}

func validateBranch(branch string) error {
	if branch == "" {
		return xerrors.New("empty branch")
	}
	if strings.HasPrefix(branch, "-") {
		return xerrors.New("branch cannot begin with dash")
	}
	return nil
}

type nopReader struct{}

func (nopReader) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (nopReader) Close() error {
	return nil
}
