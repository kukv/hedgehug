package ops

import (
	"reflect"
	"testing"
)

func TestChoiceAndConfirmRequired(t *testing.T) {
	e := choiceRequired("a store", []string{"a", "b"})
	if e.Kind != KindUsage || e.Code != "choice.required" || e.Error() != "a store is needed; pick one of: a, b" {
		t.Errorf("choiceRequired = %+v %q", e, e.Error())
	}
	if e.Args["what"] != "a store" || !reflect.DeepEqual(e.Args["candidates"], []string{"a", "b"}) {
		t.Errorf("Args = %v", e.Args)
	}
	c := confirmRequired("move ME-1")
	if c.Kind != KindUsage || c.Code != "confirm.required" || c.Error() != "move ME-1: confirm with --yes" || c.Args["what"] != "move ME-1" {
		t.Errorf("confirmRequired = %+v %q", c, c.Error())
	}
}
