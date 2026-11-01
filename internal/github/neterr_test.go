package github

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
)

// A failure to reach the host names the request once ("GET url: cause"), and
// the cause stays reachable.
func TestNetworkErrorTextHasTheURLOnce(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	api := srv.URL
	srv.Close()
	_, err := New(config.Host{API: api}, "t", nil).User()
	if err == nil {
		t.Fatal("want an error from a closed server")
	}
	if n := strings.Count(err.Error(), api); n != 1 {
		t.Errorf("the URL is in the error %d times: %q", n, err)
	}
	if !strings.HasPrefix(err.Error(), "GET "+api+"/user: ") {
		t.Errorf("error = %q; want it to start with the method and URL", err)
	}
	var op *net.OpError
	if !errors.As(err, &op) {
		t.Errorf("the *net.OpError cause is not reachable: %v", err)
	}
}
