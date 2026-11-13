package projects_test

import (
	"net/http"
	"testing"
)

func TestDeletePreviewAndDelete(t *testing.T) {
	api := newAPI(t)
	// サンプルのプロジェクトには Epic と Task の 2 件がある。
	check(t, "preview", api.Get(one+"/delete-preview"), http.StatusOK, `"items":2,"retentionDays":30`)
	check(t, "delete", api.Delete(one), http.StatusNoContent, "")
	check(t, "preview after delete", api.Get(one+"/delete-preview"), http.StatusNotFound, "")
	check(t, "delete twice", api.Delete(one), http.StatusNotFound, "")
	check(t, "delete unknown", api.Delete("/api/stores/S1/projects/01NOPE00000000000000000000"), http.StatusNotFound, "")
	check(t, "preview unknown", api.Get("/api/stores/S1/projects/01NOPE00000000000000000000/delete-preview"), http.StatusNotFound, "")
}
