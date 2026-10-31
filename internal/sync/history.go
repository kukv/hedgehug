package sync

import (
	"bytes"
	"errors"
	"io/fs"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/merge"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// sides builds the two Sides of path for a merge of ours and theirs over base.
func sides(r *gitrepo.Repo, base, ours, theirs plumbing.Hash, path string) (b []byte, o, t merge.Side, err error) {
	if b, err = readAt(r, base, path); err != nil {
		return nil, o, t, err
	}
	if o, err = side(r, base, ours, path); err != nil {
		return nil, o, t, err
	}
	t, err = side(r, base, theirs, path)
	return b, o, t, err
}

// side is path as the commit from has it, with who changed it last and
// when since base.
func side(r *gitrepo.Repo, base, from plumbing.Hash, path string) (merge.Side, error) {
	data, err := readAt(r, from, path)
	if err != nil {
		return merge.Side{}, err
	}
	s := merge.Side{Data: data}
	e, ok, err := r.LastChange(from, base, path)
	if err != nil {
		return merge.Side{}, err
	}
	if ok {
		s.Author, s.Time = author(e), e.When
	}
	if isYAML(path) {
		if s.When, err = yamlWhen(r, base, from, path); err != nil {
			return merge.Side{}, err
		}
	}
	return s, nil
}

// readAt is path at commit, or nil when it is not there.
func readAt(r *gitrepo.Repo, commit plumbing.Hash, path string) ([]byte, error) {
	b, err := r.ReadAt(commit, path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

func isYAML(path string) bool {
	return path == schema.Path || path == model.AutomationsPath || path == model.StoreMetaPath
}

// author is who made the commit: its Hedgehug-Author, else the email of
// its author.
func author(e gitrepo.LogEntry) string {
	if a, ok := e.Trailer("Hedgehug-Author"); ok {
		return a
	}
	return e.Author.Email
}

// yamlWhen tells when the side from last changed each value of the yaml
// file at path since base. A commit changes a value when the value differs
// from the one in each of its parents, so a merge that takes a value from
// one parent does not change it.
func yamlWhen(r *gitrepo.Repo, base, from plumbing.Hash, path string) (func([]string) time.Time, error) {
	es, err := r.Between(from, base)
	if err != nil {
		return nil, err
	}
	type version struct {
		data   []byte
		values map[string]string
	}
	seen := map[plumbing.Hash]version{}
	at := func(h plumbing.Hash) (version, error) {
		if v, ok := seen[h]; ok {
			return v, nil
		}
		data, err := readAt(r, h, path)
		if err != nil {
			return version{}, err
		}
		v := version{data: data, values: flatten(data)}
		seen[h] = v
		return v, nil
	}
	changed := map[string]time.Time{}
	for _, e := range es {
		cur, err := at(e.Hash)
		if err != nil {
			return nil, err
		}
		parents := make([]version, 0, len(e.Parents))
		same := false
		for _, p := range e.Parents {
			v, err := at(p)
			if err != nil {
				return nil, err
			}
			same = same || bytes.Equal(v.data, cur.data) && (v.data == nil) == (cur.data == nil)
			parents = append(parents, v)
		}
		if same {
			continue
		}
		if len(parents) == 0 {
			parents = append(parents, version{values: map[string]string{}})
		}
		keys := map[string]bool{}
		for _, v := range append(parents, cur) {
			for k := range v.values {
				keys[k] = true
			}
		}
		for k := range keys {
			now, in := cur.values[k]
			differs := true
			for _, p := range parents {
				if was, ok := p.values[k]; ok == in && was == now {
					differs = false
					break
				}
			}
			if differs && e.When.After(changed[k]) {
				changed[k] = e.When
			}
		}
	}
	return merge.WhenOf(changed), nil
}

// flatten maps each value of a yaml file, by merge.PathKey of its path, to
// its encoding. A file that does not read as yaml is one value at the root.
func flatten(data []byte) map[string]string {
	out := map[string]string{}
	if data == nil {
		return out
	}
	vs, err := merge.YAMLValues(data)
	if err != nil {
		out[merge.PathKey(nil)] = string(data)
		return out
	}
	for _, v := range vs {
		out[merge.PathKey(v.Path)] = encodeValue(v.Node)
	}
	return out
}

// encodeValue encodes n without its comments, so that a changed comment is
// not a changed value.
func encodeValue(n *yaml.Node) string {
	var x any
	if n.Decode(&x) == nil {
		if b, err := yaml.Dump(x); err == nil {
			return string(b)
		}
	}
	b, _ := yaml.Dump(n)
	return string(b)
}
