package model

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"
)

// MirroredAuthorPrefix starts the author of a comment taken from GitHub:
// github:<login>. Such a comment cannot be edited or removed in Hedgehug.
const MirroredAuthorPrefix = "github:"

// Comment is one comment on a task. The task is known from the file's path.
type Comment struct {
	ULID      ULID
	Author    string
	Created   time.Time
	Updated   time.Time
	DeletedAt *time.Time
	GH        *yaml.Node // the GitHub comments this one stands for, read-only; read them with GitHub
	Extra     Fields
	Body      string

	doc *Doc
}

var commentKeys = []string{"ulid", "author", "created", "updated", "deleted_at", "gh"}

func DecodeComment(data []byte) (*Comment, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	c := &Comment{Body: body, doc: d}
	if err := decodeAll(d, map[string]any{
		"ulid": &c.ULID, "author": &c.Author, "created": &c.Created,
		"updated": &c.Updated, "deleted_at": &c.DeletedAt, "gh": &c.GH,
	}); err != nil {
		return nil, err
	}
	if c.ULID.IsZero() {
		return nil, &DecodeError{Field: "ulid", Err: errors.New("missing")}
	}
	c.Extra = d.Extra(commentKeys)
	return c, nil
}

func (c *Comment) Encode() ([]byte, error) {
	if c.doc == nil {
		c.doc = NewDoc()
	}
	e := c.doc.Encoder()
	e.Put("ulid", c.ULID, false)
	e.Put("author", c.Author, c.Author == "")
	e.Put("created", c.Created, c.Created.IsZero())
	e.Put("updated", c.Updated, c.Updated.IsZero())
	e.Put("deleted_at", c.DeletedAt, c.DeletedAt == nil)
	e.Put("gh", c.GH, c.GH == nil)
	front, err := e.Bytes(c.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, c.Body), nil
}

// Mirrored reports whether the comment was taken from GitHub.
func (c *Comment) Mirrored() bool { return strings.HasPrefix(c.Author, MirroredAuthorPrefix) }

// CommentLink says which comment of a GitHub issue a comment stands for.
// BodySynced is the body as it was last made the same on both sides, in the
// form NormalizeBody gives; the keys it does not know stay in the file.
type CommentLink struct {
	Ref        string `yaml:"ref"` // the issue: the ref of a link of the task
	ID         int64  `yaml:"id"`  // the id of the comment on GitHub
	BodySynced string `yaml:"body_synced"`
}

// GitHub reads gh. A comment with none has no links; a gh that is not a list
// is an error, and SetGitHub would replace it.
func (c *Comment) GitHub() ([]CommentLink, error) {
	if c.GH == nil || c.GH.Tag == "!!null" {
		return nil, nil
	}
	if c.GH.Kind != yaml.SequenceNode {
		return nil, &DecodeError{Field: "gh", Err: fmt.Errorf("want a list")}
	}
	var l []CommentLink
	if err := c.GH.Decode(&l); err != nil {
		return nil, &DecodeError{Field: "gh", Err: err}
	}
	return l, nil
}

// SetGitHub writes gh and keeps, in each item that has the same ref as one
// there, the keys it does not know. With no links it leaves gh out.
func (c *Comment) SetGitHub(l []CommentLink) error {
	if len(l) == 0 {
		c.GH = nil
		if c.doc != nil {
			c.doc.drop("gh") // or Encode writes the key as null
		}
		return nil
	}
	value := new(yaml.Node)
	if err := value.Encode(l); err != nil {
		return err
	}
	if c.GH != nil && c.GH.Kind == yaml.SequenceNode {
		value = mergeByRef(c.GH, value, reflect.ValueOf(l))
	}
	c.GH = value
	return nil
}

// decodeAll decodes each key into its target. Map order does not matter:
// the first error found is returned.
func decodeAll(d *Doc, targets map[string]any) error {
	for key, out := range targets {
		if err := d.Decode(key, out); err != nil {
			return err
		}
	}
	return nil
}
