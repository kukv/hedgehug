package model

import "testing"

func TestNumbers(t *testing.T) {
	n := Numbers{"WEB": 42, "API": 7}
	b, err := n.Encode()
	if err != nil {
		t.Fatal(err)
	}
	want := "# The largest number of the purged tasks, for each id prefix. Numbering\n# never gives these numbers again.\nAPI: 7\nWEB: 42\n"
	if string(b) != want {
		t.Errorf("Encode =\n%s", b)
	}
	got, err := DecodeNumbers(b)
	if err != nil || got["WEB"] != 42 || got["API"] != 7 {
		t.Errorf("DecodeNumbers = %v %v", got, err)
	}
	if got, err := DecodeNumbers(nil); err != nil || len(got) != 0 {
		t.Errorf("empty = %v %v", got, err)
	}
	if _, err := DecodeNumbers([]byte("WEB: x\n")); err == nil {
		t.Error("a bad number decodes")
	}
}
