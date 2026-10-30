package model

import "testing"

func FuzzFrontmatter(f *testing.F) {
	for _, s := range []string{"---\na: 1\n---\nbody\n", "---\n---\n", "---\r\na: 1\r\n---", "x", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		front, body, err := splitFrontmatter([]byte(s))
		if err != nil {
			return
		}
		f2, b2, err := splitFrontmatter(joinFrontmatter(front, body))
		if err != nil || string(f2) != string(front) || b2 != body {
			t.Errorf("join/split changed %q: %q %q -> %q %q (%v)", s, front, body, f2, b2, err)
		}
	})
}

func FuzzBetween(f *testing.F) {
	for _, p := range [][2]string{{"", ""}, {"a0", "a1"}, {"Zz", "a0"}, {"a0V", ""}, {"", "Y00"}} {
		f.Add(p[0], p[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		if (a != "" && Rank(a).Validate() != nil) || (b != "" && Rank(b).Validate() != nil) || (a != "" && b != "" && a >= b) {
			return
		}
		r, err := Between(Rank(a), Rank(b))
		if err != nil {
			return // only at the ends of the key space
		}
		if r.Validate() != nil || (a != "" && string(r) <= a) || (b != "" && string(r) >= b) {
			t.Errorf("Between(%q, %q) = %q", a, b, r)
		}
	})
}

func FuzzDecodeTask(f *testing.F) {
	f.Add("---\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\ntitle: x\n---\nbody\n")
	f.Add("---\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nlabels: [a, b]\nprops: {p: 1}\n---\n")
	f.Fuzz(func(t *testing.T, s string) {
		task, err := DecodeTask([]byte(s))
		if err != nil {
			return
		}
		out, err := task.Encode()
		if err != nil {
			t.Fatalf("Encode after Decode(%q): %v", s, err)
		}
		task2, err := DecodeTask(out)
		if err != nil {
			t.Fatalf("Decode of our own output %q: %v", out, err)
		}
		out2, err := task2.Encode()
		if err != nil || string(out2) != string(out) {
			t.Errorf("Encode is not idempotent:\n%s\n---\n%s", out, out2)
		}
	})
}
