// Copyright 2023 The gg Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//		 https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"math"

	"gg-scm.io/pkg/git/githash"
	"gg-scm.io/pkg/git/object"
)

// A type that implements Repository can retrieve Git objects.
//
// OpenObject returns a reader for the object with the given hash.
// If the reader returned from OpenObject returns an EOF,
// it guarantees that the bytes read match the hash.
//
// Stat returns the same information but without returning a reader.
// This is equivalent to calling OpenObject and immediately calling Close on rc,
// but implementations may have a more efficient implementation.
type Repository interface {
	OpenObject(ctx context.Context, id githash.SHA1) (object.Prefix, io.ReadCloser, error)
	Stat(ctx context.Context, id githash.SHA1) (object.Prefix, error)
}

// A type that implements Catter can retrieve Git objects
// and dereference objects to find the object of the correct type.
type Catter interface {
	Cat(ctx context.Context, dst io.Writer, tp object.Type, id githash.SHA1) error
}

// Cat copies the content of the given object from the repository into dst.
// If the type of the object requested does not match the requested type
// and it can be trivially dereferenced to the requested type
// (e.g. a commit is found during a request for a tree),
// then the referenced object is written to dst.
//
// The given buffer will be used to store intermediate objects:
// it's assumed that reading from buf will read the bytes
// previously written to buf.
//
// If cat implements [Catter], it is used instead of reading individual objects.
// buf will not be used in this case.
func Cat(ctx context.Context, repo Repository, dst io.Writer, wantType object.Type, id githash.SHA1) error {
	if typeCat, ok := repo.(Catter); ok {
		return typeCat.Cat(ctx, dst, wantType, id)
	}

	var nextType object.Type
	buf := new(bytes.Buffer)
	for nextID := id; ; {
		got, r, err := repo.OpenObject(ctx, nextID)
		if err != nil {
			return fmt.Errorf("cat %v %v: %v", wantType, id, err)
		}
		if got.Type == wantType {
			_, err := io.Copy(dst, r)
			r.Close()
			if err != nil {
				return fmt.Errorf("cat %v %v: %v", wantType, id, err)
			}
			return nil
		}
		if nextType != "" && got.Type != nextType {
			return fmt.Errorf("cat %v %v: %v is a %v (expected %v)", wantType, id, nextID, got.Type, nextType)
		}

		switch {
		case got.Type == object.TypeCommit && wantType == object.TypeTree:
			_, err := io.Copy(buf, r)
			r.Close()
			if err != nil {
				return fmt.Errorf("cat %v %v: %v", wantType, id, err)
			}
			c, err := object.ParseCommit(buf.Bytes())
			if err != nil {
				return fmt.Errorf("cat %v %v: %v", wantType, id, err)
			}
			buf.Reset()
			nextID = c.Tree
			nextType = object.TypeTree
		case got.Type == object.TypeTag:
			_, err := io.Copy(buf, r)
			r.Close()
			if err != nil {
				return fmt.Errorf("cat %v %v: %v", wantType, id, err)
			}
			t, err := object.ParseTag(buf.Bytes())
			if err != nil {
				return fmt.Errorf("cat %v %v: %v", wantType, id, err)
			}
			buf.Reset()
			nextID = t.ObjectID
			nextType = t.ObjectType
			if !(nextType == wantType || nextType == object.TypeCommit && wantType == object.TypeTree) {
				return fmt.Errorf("cat %v %v: tag references a %v", wantType, id, nextType)
			}
		default:
			r.Close()
			return fmt.Errorf("cat %v %v: %v is a %v", wantType, id, nextID, got.Type)
		}
	}
}

// Map is an in-memory implementation of [Repository].
// The zero value is an empty repository.
type Map map[githash.SHA1]Object

// OpenObject returns the object.
func (m Map) OpenObject(ctx context.Context, id githash.SHA1) (object.Prefix, io.ReadCloser, error) {
	obj, err := m.get(ctx, id)
	if err != nil {
		return object.Prefix{}, nil, err
	}
	return obj.Prefix(), io.NopCloser(bytes.NewReader(obj.Data)), nil
}

// Stat returns the object type and size of the given key.
func (m Map) Stat(ctx context.Context, id githash.SHA1) (object.Prefix, error) {
	obj, err := m.get(ctx, id)
	if err != nil {
		return object.Prefix{}, err
	}
	return obj.Prefix(), nil
}

// Cat copies the content of the given object from the map into dst.
// If the type of the object requested does not match the requested type
// and it can be trivially dereferenced to the requested type
// (e.g. a commit is found during a request for a tree),
// then the referenced object is written to dst.
func (m Map) Cat(ctx context.Context, dst io.Writer, tp object.Type, id githash.SHA1) error {
	return Cat(ctx, onlyRepository{m}, dst, tp, id)
}

func (m Map) get(ctx context.Context, id githash.SHA1) (Object, error) {
	obj, ok := m[id]
	if !ok {
		return Object{}, fmt.Errorf("open %v: not found", id)
	}
	if id != obj.SHA1() {
		return Object{}, fmt.Errorf("open %v: corrupted", id)
	}
	return obj, nil
}

// WriteObject adds an object to the map.
func (m *Map) WriteObject(ctx context.Context, prefix object.Prefix, r io.Reader) (githash.SHA1, error) {
	if !prefix.Type.IsValid() {
		return githash.SHA1{}, fmt.Errorf("write git object: invalid type %q", prefix.Type)
	}
	if prefix.Size > math.MaxInt {
		return githash.SHA1{}, fmt.Errorf("write %v: too large (%d bytes)", prefix.Type, prefix.Size)
	}
	if prefix.Size < 0 {
		return githash.SHA1{}, fmt.Errorf("write %v: negative size", prefix.Type)
	}
	obj := Object{
		Type: prefix.Type,
		Data: make([]byte, prefix.Size),
	}
	if _, err := io.ReadFull(r, obj.Data); err != nil {
		return githash.SHA1{}, fmt.Errorf("write %v: %v", prefix.Type, err)
	}
	return m.Add(obj), nil
}

// Add adds an object to the map.
func (m *Map) Add(obj Object) githash.SHA1 {
	if *m == nil {
		*m = make(Map)
	}
	id := obj.SHA1()
	(*m)[id] = obj
	return id
}

// Object is an in-memory Git object.
type Object struct {
	Type object.Type
	Data []byte
}

// Prefix returns the object's data.
func (obj Object) Prefix() object.Prefix {
	return object.Prefix{
		Type: obj.Type,
		Size: int64(len(obj.Data)),
	}
}

// SHA1 returns the SHA-1 hash of the object.
func (obj Object) SHA1() githash.SHA1 {
	h := sha1.New()
	prefix, err := obj.Prefix().MarshalBinary()
	if err != nil {
		panic(err)
	}
	h.Write(prefix)
	h.Write(obj.Data)
	var id githash.SHA1
	h.Sum(id[:0])
	return id
}

// onlyRepository is a small wrapper type that hides any non-Repository methods.
type onlyRepository struct {
	Repository
}
