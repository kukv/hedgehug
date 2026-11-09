package replicate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/merge"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/upgrade"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// Sides builds the two Sides of path for a merge of ours and theirs over base,
// from their histories since base. The sync and the merge driver share it.
func Sides(r *gitrepo.Repo, base, ours, theirs plumbing.Hash, path string) (b []byte, o, t merge.Side, err error) {
	if b, err = readAt(r, base, path); err != nil {
		return nil, o, t, err
	}
	if b, err = UpgradedBase(r, base, ours, path, b); err != nil {
		return nil, o, t, err
	}
	if o, err = side(r, base, ours, path); err != nil {
		return nil, o, t, err
	}
	t, err = side(r, base, theirs, path)
	return b, o, t, err
}

// UpgradedBase is the file b of a task at the merge base in the store format of
// ours: a base from before a conversion is read through the steps of
// domain/upgrade, so that a side that converted and then changed a value is not
// merged against the unconverted base. Another file is b as it is. A format
// that cannot be read, or a task that does not upgrade, is an error: the merge
// does not go on against a base in another form.
func UpgradedBase(r *gitrepo.Repo, base, ours plumbing.Hash, path string, b []byte) ([]byte, error) {
	if _, err := model.ParseTaskPath(path); err != nil || b == nil {
		return b, nil //nolint:nilerr // not a task file
	}
	to, err := FormatAt(r, ours)
	if err != nil {
		return nil, err
	}
	up, err := upgrade.At(func(p string) ([]byte, error) { return r.ReadAt(base, p) }).TaskTo(to, b)
	if err != nil {
		return nil, fmt.Errorf("%s at the merge base: %w", path, err)
	}
	return up, nil
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
		s.Author, s.Time = e.Writer(), e.When
	}
	if merge.ByKey(path) {
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
		values, err := flatten(data)
		if err != nil {
			return version{}, err
		}
		v := version{data: data, values: values}
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
func flatten(data []byte) (map[string]string, error) {
	out := map[string]string{}
	if data == nil {
		return out, nil
	}
	vs, err := merge.YAMLValues(data)
	if err != nil {
		out[merge.PathKey(nil)] = string(data)
		return out, nil
	}
	for _, v := range vs {
		s, err := encodeValue(v.Node)
		if err != nil {
			return nil, err
		}
		out[merge.PathKey(v.Path)] = s
	}
	return out, nil
}

// encodeValue encodes n without its comments, so that a changed comment is
// not a changed value. A value with no Go form, such as a mapping with keys
// that are not strings, is encoded as its node, comments and all.
func encodeValue(n *yaml.Node) (string, error) {
	var x any
	if n.Decode(&x) == nil {
		b, err := yaml.Dump(x)
		return string(b), err
	}
	b, err := yaml.Dump(n)
	return string(b), err
}
