package retrydedupe

import (
	"bytes"
	"errors"
	"testing"
)

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

func TestAlikeNewOrdersNotSwallowed(t *testing.T) {
	calls := 0
	down := func(req []byte) ([]byte, error) {
		calls++
		return append([]byte("ok:"), req...), nil
	}
	reqs := [][]byte{[]byte("10:aaa"), []byte("10:aab"), []byte("11:aaa")}
	out, err := Handle(reqs, down)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("alike new orders must each compute, calls=%d", calls)
	}
	want := [][]byte{[]byte("ok:10:aaa"), []byte("ok:10:aab"), []byte("ok:11:aaa")}
	if len(out) != len(want) {
		t.Fatalf("want %d entries, got %#v", len(want), out)
	}
	for i := range want {
		if !bytes.Equal(out[i], want[i]) {
			t.Fatalf("out[%d]=%q want %q", i, out[i], want[i])
		}
	}
}

func TestJitterDoesNotRecomputeCommitted(t *testing.T) {
	calls := 0
	down := func(req []byte) ([]byte, error) {
		calls++
		// downstream committed the entry, then the wire jittered
		return append([]byte("ok:"), req...), errors.New("jitter")
	}
	out, err := Handle([][]byte{[]byte("10:aaa")}, down)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("committed order must not re-execute, calls=%d", calls)
	}
	if len(out) != 1 || !bytes.Equal(out[0], []byte("ok:10:aaa")) {
		t.Fatalf("want committed entry, got %#v", out)
	}
}

func TestHalfReplyDroppedGoodSeqsKeepOrder(t *testing.T) {
	down := func(req []byte) ([]byte, error) {
		if bytes.Equal(req, []byte("11:bbb")) {
			return []byte("ok:1"), errors.New("cut off") // half reply
		}
		return append([]byte("ok:"), req...), nil
	}
	reqs := [][]byte{[]byte("10:aaa"), []byte("11:bbb"), []byte("12:ccc")}
	out, err := Handle(reqs, down)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]byte{[]byte("ok:10:aaa"), []byte("ok:12:ccc")}
	if len(out) != len(want) {
		t.Fatalf("half reply must be dropped, got %#v", out)
	}
	for i := range want {
		if !bytes.Equal(out[i], want[i]) {
			t.Fatalf("out[%d]=%q want %q", i, out[i], want[i])
		}
	}
	gotSeqs := []int{DecodeSeq(out[0][3:]), DecodeSeq(out[1][3:])}
	if gotSeqs[0] != 10 || gotSeqs[1] != 12 {
		t.Fatalf("good seqs must stay in order, got %v", gotSeqs)
	}
}

func TestRetryRecoversFromEmptyFailure(t *testing.T) {
	calls := 0
	down := func(req []byte) ([]byte, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("downstream down")
		}
		return append([]byte("ok:"), req...), nil
	}
	out, err := Handle([][]byte{[]byte("10:aaa")}, down)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("want 3 tries, calls=%d", calls)
	}
	if len(out) != 1 || !bytes.Equal(out[0], []byte("ok:10:aaa")) {
		t.Fatalf("want retried entry, got %#v", out)
	}
}
