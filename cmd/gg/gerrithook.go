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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"gg-scm.io/pkg/internal/flag"
)

const gerrithookSynopsis = "install or uninstall Gerrit change ID hook"

const commitMsgHookDefaultURL = "https://gerrit-review.googlesource.com/tools/hooks/commit-msg"

func gerrithook(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg gerrithook [-url=URL] [ on | off ]", gerrithookSynopsis+`

	The Gerrit change ID hook is a commit message hook which automatically
	inserts a globally unique Change-Id tag in the footer of a commit
	message.

	gerrithook downloads the hook script from a public Gerrit server. gg
	caches the last successfully fetched hook script for each URL in
	`+"`$XDG_CACHE_DIR/gg/gerrithook/`"+`, so if the server is unavailable,
	the local file is used. `+"`-cached`"+` can force using the cached file.

	More details at https://gerrit-review.googlesource.com/hooks/commit-msg`)
	url := f.String("url", commitMsgHookDefaultURL, "URL of hook script to download")
	cacheOnly := f.Bool("cached", false, "Use local cache instead of downloading")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 1 {
		return usagef("too many arguments to gerrithook")
	}
	switch f.Arg(0) {
	case "", "on":
		return installGerritHook(ctx, cc, *url, *cacheOnly)
	case "off":
		return uninstallGerritHook(ctx, cc)
	default:
		return usagef("gerrithook argument must be either 'on' or 'off'")
	}
}

func installGerritHook(ctx context.Context, cc *cmdContext, url string, cacheOnly bool) error {
	// Determine destination path first.
	// This is relatively cheap, so if it fails, we want to fail early.
	cfg, err := cc.git.ReadConfig(ctx)
	if err != nil {
		return fmt.Errorf("install gerrit hook: %w", err)
	}
	path, err := commitMsgHookPath(ctx, cfg, cc.git)
	if err != nil {
		return fmt.Errorf("install gerrit hook: %w", err)
	}

	// Open script source.
	var scriptSource io.ReadCloser
	writeCache := false
	if !cacheOnly {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			fmt.Fprintf(cc.stderr, "gg: install gerrit hook: %v\n", err)
			goto readCached
		}
		req.Header.Set("User-Agent", userAgentString())
		resp, err := cc.httpClient.Do(req.WithContext(ctx))
		if err != nil {
			fmt.Fprintf(cc.stderr, "gg: install gerrit hook: %s returned %v\n", url, err)
			goto readCached
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			fmt.Fprintf(cc.stderr, "gg: install gerrit hook: %s returned HTTP %s\n", url, resp.Status)
			goto readCached
		}
		scriptSource = &limitedReader{resp.Body, 1 << 20} // 1MiB limit
		writeCache = true
	}
readCached:
	urlSum := sha256.Sum256([]byte(url))
	cacheName := "gerrithook/" + hex.EncodeToString(urlSum[:])
	if scriptSource == nil {
		// Either cache-only was requested or we were unable to read from the URL.
		cacheFile, err := cc.xdgDirs.openCache(cacheName)
		if err != nil {
			return fmt.Errorf("install gerrit hook: %w", err)
		}
		scriptSource = cacheFile
	}
	defer scriptSource.Close()

	// Back up old hook.
	dst := filepath.Join(filepath.Dir(path), "commit-msg.old")
	if err := os.Rename(path, dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("install gerrit hook: %w", err)
	}
	// Write to hook location in repository, caching if we succeed.
	flags := os.O_CREATE | os.O_EXCL
	if writeCache {
		flags |= os.O_RDWR
	} else {
		flags |= os.O_WRONLY
	}
	f, err := os.OpenFile(path, flags, 0777)
	if err != nil {
		return fmt.Errorf("install gerrit hook: %w", err)
	}
	_, cpErr := io.Copy(f, scriptSource)
	if cpErr == nil && writeCache {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			fmt.Fprintf(cc.stderr, "gg: could not cache gerrit hook: %v\n", err)
			goto closeHook
		}
		cacheFile, err := cc.xdgDirs.createCache(cacheName)
		if err != nil {
			fmt.Fprintf(cc.stderr, "gg: could not cache gerrit hook: %v\n", err)
			goto closeHook
		}
		cacheFilePath := cacheFile.Name()
		_, cacheCpErr := io.Copy(cacheFile, f)
		closeCacheErr := cacheFile.Close()
		if cacheCpErr != nil {
			fmt.Fprintf(cc.stderr, "gg: could not cache gerrit hook: %v\n", cacheCpErr)
			os.Remove(cacheFilePath)
			goto closeHook
		}
		if closeCacheErr != nil {
			fmt.Fprintf(cc.stderr, "gg: could not cache gerrit hook: %v\n", closeCacheErr)
			os.Remove(cacheFilePath)
			goto closeHook
		}
	}
closeHook:
	closeErr := f.Close()
	if cpErr != nil {
		return fmt.Errorf("install gerrit hook: %w", cpErr)
	}
	if closeErr != nil {
		return fmt.Errorf("install gerrit hook: %w", closeErr)
	}
	return nil
}

func uninstallGerritHook(ctx context.Context, cc *cmdContext) error {
	cfg, err := cc.git.ReadConfig(ctx)
	if err != nil {
		return fmt.Errorf("install gerrit hook: %w", err)
	}
	path, err := commitMsgHookPath(ctx, cfg, cc.git)
	if err != nil {
		return fmt.Errorf("uninstall gerrit hook: %w", err)
	}
	dst := filepath.Join(filepath.Dir(path), "commit-msg.old")
	if err := os.Rename(path, dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("uninstall gerrit hook: %w", err)
	}
	return nil
}

type valuer interface {
	Value(string) string
	Bool(string) (bool, error)
}

type gitDirs interface {
	CommonDir(ctx context.Context) (string, error)
	WorkTree(ctx context.Context) (string, error)
}

func commitMsgHookPath(ctx context.Context, cfg valuer, g gitDirs) (string, error) {
	// TODO(someday): Move hook directory path logic into internal/git.

	path := cfg.Value("core.hooksPath")
	if path == "" {
		commonDir, err := g.CommonDir(ctx)
		if err != nil {
			return "", err
		}
		return filepath.Join(commonDir, "hooks", "commit-msg"), nil
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	if bare, err := cfg.Bool("core.bare"); err != nil {
		return "", err
	} else if bare {
		commonDir, err := g.CommonDir(ctx)
		if err != nil {
			return "", err
		}
		return filepath.Join(commonDir, path, "commit-msg"), nil
	}
	topDir, err := g.WorkTree(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(topDir, path, "commit-msg"), nil
}

type limitedReader struct {
	R io.ReadCloser // underlying reader
	N int64         // max bytes remaining
}

func (l *limitedReader) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		return 0, errors.New("read limit reached")
	}
	if int64(len(p)) > l.N {
		p = p[0:l.N]
	}
	n, err = l.R.Read(p)
	l.N -= int64(n)
	return
}

func (l *limitedReader) Close() error {
	return l.R.Close()
}

type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (nopWriteCloser) Close() error {
	return nil
}
