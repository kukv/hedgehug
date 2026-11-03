package model

import "go.yaml.in/yaml/v4"

// ListStyle tells the style of the block list v under the key k: flush
// when its items are written at the column of the key ("k:\n- a"), not when
// they are indented ("k:\n  - a"). An item starts after "- ", so it is 2
// columns right of a flush list and more than 2 right of an indented one.
// ok is false when v is not a block list with items.
func ListStyle(k, v *yaml.Node) (flush, ok bool) {
	if v.Kind != yaml.SequenceNode || v.Style&yaml.FlowStyle != 0 || len(v.Content) == 0 {
		return false, false
	}
	return v.Content[0].Column-k.Column <= 2, true
}

// FirstListStyle is the ListStyle of the first block list under a key in n,
// in document order. ok is false when there is none.
func FirstListStyle(n *yaml.Node) (flush, ok bool) {
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if flush, ok := ListStyle(n.Content[i], n.Content[i+1]); ok {
				return flush, true
			}
			if flush, ok := FirstListStyle(n.Content[i+1]); ok {
				return flush, true
			}
		}
	case yaml.SequenceNode:
		for _, it := range n.Content {
			if flush, ok := FirstListStyle(it); ok {
				return flush, true
			}
		}
	}
	return false, false
}
