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
	"os"
	"strings"

	"zombiezen.com/go/gut/internal/flag"
	"zombiezen.com/go/gut/internal/gittool"
)

const statusSynopsis = "show changed files in the working directory"

func status(ctx context.Context, git *gittool.Tool, args []string) error {
	f := flag.NewFlagSet(true, "gut status [FILE [...]]", statusSynopsis)
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(os.Stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	p, err := git.Start(ctx, append([]string{"status", "--porcelain=v1", "-z", "-unormal", "--"}, f.Args()...)...)
	if err != nil {
		return err
	}
	defer p.Wait()
	r := bufio.NewReader(p)
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
			fmt.Println("M", ent.name)
		case ent.isAdded():
			fmt.Println("A", ent.name)
		case ent.isRemoved():
			fmt.Println("R", ent.name)
		case ent.isCopied():
			fmt.Printf("A %s\n  %s\n", ent.name, ent.from)
		case ent.isRenamed():
			fmt.Printf("A %s\n  %s\nR %s\n", ent.name, ent.from, ent.from)
		case ent.isMissing():
			fmt.Println("!", ent.name)
		case ent.isUntracked():
			fmt.Println("?", ent.name)
		case ent.isIgnored():
			fmt.Println("I", ent.name)
		case ent.isUnmerged():
			fmt.Println("U", ent.name)
		default:
			panic("unreachable")
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
