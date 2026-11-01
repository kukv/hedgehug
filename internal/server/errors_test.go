package server

import "testing"

// TestUnknownStoreOnEveryOperation covers the error case of the operations
// that take ?store=: each answers store.not_found.
func TestUnknownStoreOnEveryOperation(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("one", "")
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/schema"},
		{"GET", "/api/schema/check"},
		{"POST", "/api/schema/migrate"},
		{"GET", "/api/doctor"},
		{"POST", "/api/doctor/fix"},
		{"GET", "/api/views"},
		{"GET", "/api/views/w"},
		{"POST", "/api/sync"},
		{"GET", "/api/activity"},
		{"GET", "/api/tasks/" + id},
	} {
		var body any
		if c.method == "POST" {
			body = map[string]any{"target": "status", "old": "a", "new": "b"}
		}
		if p := ts.problem(c.method, c.path+"?store=nope", body, 404); p.Type != "store.not_found" {
			t.Errorf("%s %s: problem = %+v", c.method, c.path, p)
		}
	}
}

func TestRestoreErrors(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("one", "")
	ts.problem("POST", "/api/tasks/ME-99/restore", nil, 404)
	ts.problem("POST", "/api/tasks/ME-99/history/0000000000000000000000000000000000000000/restore", nil, 404)
	if p := ts.problem("POST", "/api/tasks/"+id+"/history/zzzz/restore", nil, 422); p.Type != "restore_at.bad_time" {
		t.Errorf("bad commit: %+v", p)
	}
}
