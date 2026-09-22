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

func TestJitterFullResponseComputesOnce(t *testing.T) {
	calls := 0
	down := func(req []byte) ([]byte, error) {
		calls++
		switch calls {
		case 1, 2:
			return nil, errJitter
		default:
			return append([]byte("ok:"), req...), nil
		}
	}
	out, err := Handle([][]byte{[]byte("10:aaa")}, down)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || string(out[0]) != "ok:10:aaa" {
		t.Fatalf("want single full response ok:10:aaa, got %#v", out)
	}
	if calls != 3 {
		t.Fatalf("want 3 attempts through jitter, calls=%d", calls)
	}
}

func TestFullResponseStopsRetries(t *testing.T) {
	calls := 0
	down := func(req []byte) ([]byte, error) {
		calls++
		return append([]byte("ok:"), req...), nil
	}
	out, _ := Handle([][]byte{[]byte("10:aaa"), []byte("10:aaa")}, down)
	if calls != 1 {
		t.Fatalf("full response must stop retries and dedupe repeat, calls=%d", calls)
	}
	if len(out) != 1 || string(out[0]) != "ok:10:aaa" {
		t.Fatalf("want one ledger entry, got %#v", out)
	}
}

func TestAlikeNewOrdersNotSwallowed(t *testing.T) {
	calls := 0
	down := func(req []byte) ([]byte, error) {
		calls++
		return append([]byte("ok:"), req...), nil
	}
	// All three share the same first two bytes; prefix-key dedupe would merge them.
	reqs := [][]byte{[]byte("10:aaa"), []byte("10:aab"), []byte("10:aac")}
	out, err := Handle(reqs, down)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("alike new orders must all be posted, got %#v", out)
	}
	if calls != 3 {
		t.Fatalf("want 3 downstream calls, calls=%d", calls)
	}
}

func TestHalfResponseDroppedKeepsLaterSeq(t *testing.T) {
	calls := 0
	down := func(req []byte) ([]byte, error) {
		calls++
		if string(req) == "10:aaa" {
			// nil error but a truncated answer for the first order.
			return []byte("ok:1"), nil
		}
		return append([]byte("ok:"), req...), nil
	}
	reqs := [][]byte{[]byte("10:aaa"), []byte("11:bbb")}
	out, err := Handle(reqs, down)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || string(out[0]) != "ok:11:bbb" {
		t.Fatalf("half response dropped, later good order kept, got %#v", out)
	}
	if calls != 4 {
		t.Fatalf("want 3 retries for the half order plus 1 good call, calls=%d", calls)
	}
}

func TestResponsesInOriginalOrder(t *testing.T) {
	down := func(req []byte) ([]byte, error) {
		return append([]byte("ok:"), req...), nil
	}
	reqs := [][]byte{[]byte("12:a"), []byte("12:a"), []byte("10:b"), []byte("11:c")}
	out, err := Handle(reqs, down)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ok:12:a", "ok:10:b", "ok:11:c"}
	if len(out) != len(want) {
		t.Fatalf("want %d entries, got %#v", len(want), out)
	}
	for i, w := range want {
		if string(out[i]) != w {
			t.Fatalf("entry %d: want %s, got %s", i, w, out[i])
		}
	}
}

var errJitter = jitterErr{}

type jitterErr struct{}

func (jitterErr) Error() string { return "jitter" }
