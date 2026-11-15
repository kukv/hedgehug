package search

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

func TestFillValuesCopiesChoicesAndFields(t *testing.T) {
	it := item.Item{
		Choices: map[change.ID][]change.ID{"pSel": {"v1", "v2"}, "pOne": {"v3"}},
		Fields:  map[change.ID]json.RawMessage{"pText": json.RawMessage(`"本文"`), "pNum": json.RawMessage(`3.5`)},
	}
	var row SearchRow
	fillValues(&row, it)
	v := row.Values
	if len(v.Choices["pSel"]) != 2 || v.Choices["pSel"][0] != "v1" || v.Choices["pSel"][1] != "v2" || len(v.Choices["pOne"]) != 1 || v.Choices["pOne"][0] != "v3" {
		t.Errorf("choices = %v", v.Choices)
	}
	if string(v.Fields["pText"]) != `"本文"` || string(v.Fields["pNum"]) != `3.5` {
		t.Errorf("fields = %v", v.Fields)
	}
}
