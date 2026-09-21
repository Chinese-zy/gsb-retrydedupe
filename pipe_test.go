package retrydedupe

import "testing"

func TestSameOrderOnceNewNotSwallowed(t *testing.T) {
	calls := 0
	down := func(req []byte) ([]byte, error) {
		calls++
		return append([]byte("ok:"), req...), nil
	}
	reqs := [][]byte{[]byte("10:aaa"), []byte("10:aaa"), []byte("11:bbb")}
	out, err := Handle(reqs, down)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2 unique orders, got %d %#v", len(out), out)
	}
	if calls != 2 {
		t.Fatalf("same order should compute once, calls=%d", calls)
	}
}
