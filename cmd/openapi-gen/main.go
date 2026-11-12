// Command openapi-gen は、httpapi の API の OpenAPI を、引数のファイルに書く(make generate)。
package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: openapi-gen <output.yaml>")
		os.Exit(2)
	}
	api := httpapi.NewAPI(http.NewServeMux(), httpapi.Deps{Version: "dev"})
	// orval は OpenAPI 3.0 を読むので、3.1 から下げて書く。
	b, err := api.OpenAPI().DowngradeYAML()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(os.Args[1], b, 0o644); err != nil { //nolint:gosec // 生成物はリポジトリに入れる公開のファイル
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
