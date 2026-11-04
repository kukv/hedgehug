package schemas_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
)

// No findings are [] in JSON, not null.
func TestCheckResultOfNothingIsAnEmptyList(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(schemas.CheckResultOf(nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"findings":[]}` {
		t.Errorf("json = %s", b)
	}
}
