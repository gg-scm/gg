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

	gerrithook downloads the hook script from a public Gerrit server.

	More details at https://gerrit-review.googlesource.com/hooks/commit-msg`)
	url := f.String("url", commitMsgHookDefaultURL, "URL of hook script to download")
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
		return installGerritHook(ctx, cc, *url)
	case "off":
		return uninstallGerritHook(ctx, cc)
	default:
		return usagef("gerrithook argument must be either 'on' or 'off'")
	}
}

func installGerritHook(ctx context.Context, cc *cmdContext, url string) error {
	path, err := commitMsgHookPath(ctx, cc)
	if err != nil {
		return fmt.Errorf("install gerrit hook: %v", err)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("install gerrit hook: %v", err)
	}
	req.Header.Set("User-Agent", userAgentString())
	resp, err := cc.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("install gerrit hook: %s returned %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("install gerrit hook: %s returned HTTP %s", url, resp.Status)
	}

	dst := filepath.Join(filepath.Dir(path), "commit-msg.old")
	if err := os.Rename(path, dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("install gerrit hook: %v", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0777)
	if err != nil {
		return fmt.Errorf("install gerrit hook: %v", err)
	}
	_, cpErr := io.Copy(f, &limitedReader{resp.Body, 1 << 20}) // 1MiB limit
	closeErr := f.Close()
	if cpErr != nil {
		return fmt.Errorf("install gerrit hook: %v", cpErr)
	}
	if closeErr != nil {
		return fmt.Errorf("install gerrit hook: %v", closeErr)
	}
	return nil
}

func uninstallGerritHook(ctx context.Context, cc *cmdContext) error {
	path, err := commitMsgHookPath(ctx, cc)
	if err != nil {
		return fmt.Errorf("uninstall gerrit hook: %v", err)
	}
	dst := filepath.Join(filepath.Dir(path), "commit-msg.old")
	if err := os.Rename(path, dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("uninstall gerrit hook: %v", err)
	}
	return nil
}

func commitMsgHookPath(ctx context.Context, cc *cmdContext) (string, error) {
	commonBytes, err := cc.git.RunOneLiner(ctx, '\n', "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	common := cc.abs(string(commonBytes))
	return filepath.Join(common, "hooks", "commit-msg"), nil
}

type limitedReader struct {
	R io.Reader // underlying reader
	N int64     // max bytes remaining
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
