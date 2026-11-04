package usecase_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
)

func TestChoiceAndConfirmRequired(t *testing.T) {
	t.Parallel()
	e := usecase.ChoiceRequired("a store", []string{"a", "b"})
	if e.Kind != usecase.KindUsage || e.Code != "choice.required" || e.Error() != "a store is needed; pick one of: a, b" {
		t.Errorf("choiceRequired = %+v %q", e, e.Error())
	}
	if e.Args["what"] != "a store" || !reflect.DeepEqual(e.Args["candidates"], []string{"a", "b"}) {
		t.Errorf("Args = %v", e.Args)
	}
	c := usecase.ConfirmRequired("move ME-1")
	if c.Kind != usecase.KindUsage || c.Code != "confirm.required" || c.Error() != "move ME-1: confirm with --yes" || c.Args["what"] != "move ME-1" {
		t.Errorf("confirmRequired = %+v %q", c, c.Error())
	}
}
