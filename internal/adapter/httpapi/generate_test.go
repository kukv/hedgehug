package httpapi_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
)

// api/openapi.yaml は、いまの API から作ったものと同じでなければならない(make generate を忘れると落ちる)。
func TestOpenAPIYAMLIsUpToDate(t *testing.T) {
	want, err := httpapi.OpenAPIYAML()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("api/openapi.yaml is stale; run make generate. want:\n%s", want)
	}
}
