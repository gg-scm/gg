// Copyright 2019 Google LLC
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
	"context"
	"fmt"
)

// ListRemoteRefs lists all of the refs in a remote repository.
// remote may be a URL or the name of a remote.
//
// This function may block on user input if the remote requires
// credentials.
func (g *Git) ListRemoteRefs(ctx context.Context, remote string) (map[Ref]Hash, error) {
	// TODO(now): Add tests.

	errPrefix := fmt.Sprintf("git ls-remote %q", remote)
	out, err := g.run(ctx, errPrefix, []string{g.exe, "ls-remote", "--quiet", "--", remote})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	refs, err := parseRefs(out)
	if err != nil {
		return refs, fmt.Errorf("%s: %v", errPrefix, err)
	}
	return refs, nil
}
