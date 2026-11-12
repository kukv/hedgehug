package httpapi

import "net/http"

//go:generate go run ../../../cmd/openapi-gen ../../../api/openapi.yaml

// OpenAPIYAML は、api/openapi.yaml に書く OpenAPI(make generate)。orval は OpenAPI 3.0 を読むので、
// 3.1 から下げて書く。
func OpenAPIYAML() ([]byte, error) {
	return NewAPI(http.NewServeMux(), Deps{Version: "dev"}).OpenAPI().DowngradeYAML()
}
