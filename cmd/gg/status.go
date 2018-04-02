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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gittool"
	"zombiezen.com/go/gg/internal/terminal"
)

const statusSynopsis = "show changed files in the working directory"

func status(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg status [FILE [...]]", statusSynopsis)
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
		ignoredColor   []byte
		untrackedColor []byte
		unmergedColor  []byte
	)
	colorize, err := gittool.ColorBool(ctx, cc.git, "color.ggstatus", terminal.IsTerminal(cc.stdout))
	if err != nil {
		fmt.Fprintln(cc.stderr, "gg:", err)
	} else if colorize {
		addedColor, err = gittool.Color(ctx, cc.git, "color.ggstatus.added", "green")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
		modifiedColor, err = gittool.Color(ctx, cc.git, "color.ggstatus.modified", "blue")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
		removedColor, err = gittool.Color(ctx, cc.git, "color.ggstatus.removed", "red")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
		missingColor, err = gittool.Color(ctx, cc.git, "color.ggstatus.deleted", "cyan")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
		untrackedColor, err = gittool.Color(ctx, cc.git, "color.ggstatus.unknown", "magenta")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
		ignoredColor, err = gittool.Color(ctx, cc.git, "color.ggstatus.ignored", "black")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
		unmergedColor, err = gittool.Color(ctx, cc.git, "color.ggstatus.unmerged", "blue")
		if err != nil {
			fmt.Fprintln(cc.stderr, "gg:", err)
		}
	}
	p, err := cc.git.Start(ctx, append([]string{"status", "--porcelain", "-z", "-unormal", "--"}, f.Args()...)...)
	if err != nil {
		return err
	}
	defer p.Wait()
	r := bufio.NewReader(p)
	if colorize {
		if err := terminal.ResetTextStyle(cc.stdout); err != nil {
			return err
		}
	}
	for {
		ent, err := readStatusEntry(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch {
		case ent.isModified():
			_, err = fmt.Fprintf(cc.stdout, "%sM %s\n", modifiedColor, ent.name)
		case ent.isAdded():
			_, err = fmt.Fprintf(cc.stdout, "%sA %s\n", addedColor, ent.name)
		case ent.isRemoved():
			_, err = fmt.Fprintf(cc.stdout, "%sR %s\n", removedColor, ent.name)
		case ent.isCopied():
			if _, err := fmt.Fprintf(cc.stdout, "%sA %s\n", addedColor, ent.name); err != nil {
				return err
			}
			if colorize {
				if err := terminal.ResetTextStyle(cc.stdout); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintf(cc.stdout, "  %s\n", ent.from)
		case ent.isRenamed():
			fmt.Fprintf(cc.stdout, "%sA %s\n", addedColor, ent.name)
			if colorize {
				if err := terminal.ResetTextStyle(cc.stdout); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintf(cc.stdout, "  %s\n%sR %s\n", ent.from, removedColor, ent.from)
		case ent.isMissing():
			_, err = fmt.Fprintf(cc.stdout, "%s! %s\n", missingColor, ent.name)
		case ent.isUntracked():
			_, err = fmt.Fprintf(cc.stdout, "%s? %s\n", untrackedColor, ent.name)
		case ent.isIgnored():
			_, err = fmt.Fprintf(cc.stdout, "%sI %s\n", ignoredColor, ent.name)
		case ent.isUnmerged():
			_, err = fmt.Fprintf(cc.stdout, "%sU %s\n", unmergedColor, ent.name)
		default:
			panic("unreachable")
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
	return p.Wait()
}

type statusEntry struct {
	code [2]byte
	name string
	from string
}

func (ent *statusEntry) isMissing() bool {
	return ent.code[1] == 'D'
}

func (ent *statusEntry) isModified() bool {
	return ent.code[0] == 'M' && ent.code[1] == ' ' ||
		ent.code[0] == ' ' && ent.code[1] == 'M' ||
		ent.code[0] == 'M' && ent.code[1] == 'M'
}

func (ent *statusEntry) isRemoved() bool {
	return ent.code[0] == 'D' && ent.code[1] == ' '
}

func (ent *statusEntry) isRenamed() bool {
	return ent.code[0] == 'R' && (ent.code[1] == ' ' || ent.code[1] == 'M')
}

func (ent *statusEntry) isCopied() bool {
	return ent.code[0] == 'C' && (ent.code[1] == ' ' || ent.code[1] == 'M')
}

func (ent *statusEntry) isAdded() bool {
	return ent.code[0] == 'A' && (ent.code[1] == ' ' || ent.code[1] == 'M') ||
		ent.code[0] == ' ' && ent.code[1] == 'A'
}

func (ent *statusEntry) isUntracked() bool {
	return ent.code[0] == '?' && ent.code[1] == '?'
}

func (ent *statusEntry) isIgnored() bool {
	return ent.code[0] == '!' && ent.code[1] == '!'
}

func (ent *statusEntry) isUnmerged() bool {
	return ent.code[0] == 'D' && ent.code[1] == 'D' ||
		ent.code[0] == 'A' && ent.code[1] == 'U' ||
		ent.code[0] == 'U' && ent.code[1] == 'D' ||
		ent.code[0] == 'U' && ent.code[1] == 'A' ||
		ent.code[0] == 'D' && ent.code[1] == 'U' ||
		ent.code[0] == 'A' && ent.code[1] == 'A' ||
		ent.code[0] == 'U' && ent.code[1] == 'U'
}

func readStatusEntry(r io.ByteReader) (*statusEntry, error) {
	ent := new(statusEntry)
	var err error
	ent.code[0], err = r.ReadByte()
	if err != nil {
		return nil, err
	}
	ent.code[1], err = r.ReadByte()
	if err == io.EOF {
		return nil, errors.New("read status entry: unexpected EOF")
	} else if err != nil {
		return nil, fmt.Errorf("read status entry: %v", err)
	}
	if sp, err := r.ReadByte(); err == io.EOF {
		return nil, errors.New("read status entry: unexpected EOF")
	} else if err != nil {
		return nil, fmt.Errorf("read status entry: %v", err)
	} else if sp != ' ' {
		return nil, fmt.Errorf("read status entry: expected ' ', got %q", sp)
	}
	ent.name, err = readString(r, 2048)
	if err != nil {
		return nil, fmt.Errorf("read status entry: %v", err)
	}
	if ent.code[0] == 'R' || ent.code[0] == 'C' {
		ent.from, err = readString(r, 2048)
		if err != nil {
			return nil, fmt.Errorf("read status entry: %v", err)
		}
	}
	// Check at very end in order to consume as much as possible.
	if !isValidStatusCode(ent.code) {
		return nil, fmt.Errorf("read status entry: invalid code %q %q", ent.code[0], ent.code[1])
	}
	return ent, nil
}

func isValidStatusCode(code [2]byte) bool {
	const codes = "??!!" +
		" M D A" +
		"M MMMD" +
		"A AMAD" +
		"D " +
		"R RMRD" +
		"C CMCD" +
		"DDAUUDUADUAAUU"
	for i := 0; i < len(codes); i += 2 {
		if code[0] == codes[i] && code[1] == codes[i+1] {
			return true
		}
	}
	return false
}

// readString reads a NUL-terminated string from r.
func readString(r io.ByteReader, limit int) (string, error) {
	var sb strings.Builder
	for sb.Len() < limit {
		b, err := r.ReadByte()
		if err == io.EOF {
			return "", io.ErrUnexpectedEOF
		}
		if err != nil {
			return "", err
		}
		if b == 0 {
			return sb.String(), nil
		}
		sb.WriteByte(b)
	}
	b, err := r.ReadByte()
	if err == io.EOF {
		return "", io.ErrUnexpectedEOF
	}
	if err != nil {
		return "", err
	}
	if b != 0 {
		return "", errors.New("string too long")
	}
	return sb.String(), nil
}
