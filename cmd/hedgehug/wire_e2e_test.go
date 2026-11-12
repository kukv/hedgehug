//go:build e2e

package main

import "testing"

// e2e のビルドは、このマシンの中(ループバック)でしか待ち受けない。
func TestE2EAddrMustBeLoopback(t *testing.T) {
	for addr, ok := range map[string]bool{
		"127.0.0.1:0":    true,
		"127.0.0.1:5173": true,
		"[::1]:0":        true,
		"localhost:8080": true,
		":8080":          false,
		"0.0.0.0:8080":   false,
		"[::]:8080":      false,
		"192.168.1.2:80": false,
		"example.com:80": false,
		"127.0.0.1":      false,
		"":               false,
	} {
		if err := checkLoopback(addr); (err == nil) != ok {
			t.Errorf("checkLoopback(%q) = %v", addr, err)
		}
	}
}

func TestWireRefusesNonLoopbackAddr(t *testing.T) {
	t.Setenv("HEDGEHUG_TOKEN", "secret")
	t.Setenv("HEDGEHUG_ADDR", "0.0.0.0:8080")
	if _, err := wire(t.Context()); err == nil {
		t.Error("wire accepted a non-loopback address")
	}
}
