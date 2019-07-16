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

// cmd.exe doesn't support ANSI escape sequences until Windows 10.
// Let's pick the safer option of not colorizing.

// Color returns the ANSI escape sequence for the given configuration
// setting.
func (cfg *Config) Color(name string, default_ string) ([]byte, error) {
	return nil, nil
}

// ColorBool finds the color configuration setting is true or false.
// isTerm indicates whether the eventual output will be a terminal.
func (cfg *Config) ColorBool(name string, isTerm bool) (bool, error) {
	return false, nil
}
