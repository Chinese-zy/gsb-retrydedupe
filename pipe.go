package retrydedupe

import "bytes"

// Handle is the public entry. It wires retry, dedupe and the ledger together
// without changing the public request/response byte format:
// requests look like "10:aaa" ("seq:body"), responses look like "ok:10:aaa".
func Handle(reqs [][]byte, down func([]byte) ([]byte, error)) ([][]byte, error) {
	deduper := newDeduper()
	ledger := newLedger()
	for _, req := range reqs {
		// Dedupe: identity is the full request bytes, never a prefix.
		if deduper.isSeen(req) {
			continue
		}

		// Retry: keep calling downstream on jitter, but stop as soon as a
		// full response lands; a half response is discarded, not posted.
		resp, ok := withRetry(req, down, 3, fullResponse)
		if !ok {
			continue
		}

		// Ledger: an order is posted exactly once, in original request order.
		deduper.mark(req)
		ledger.post(resp)
	}
	return ledger.items(), nil
}

// withRetry calls down until it returns a full response (accepted=true) or
// attempts are exhausted. A half response (nil error but truncated bytes, or
// an error response) is dropped without reaching the ledger.
func withRetry(req []byte, down func([]byte) ([]byte, error), attempts int, complete func(req, resp []byte) bool) ([]byte, bool) {
	for try := 0; try < attempts; try++ {
		resp, err := down(req)
		if err != nil {
			continue
		}
		if complete(req, resp) {
			return resp, true
		}
	}
	return nil, false
}

// fullResponse validates the wire format without changing it: a full
// downstream answer echoes the whole request ("ok:10:aaa" for "10:aaa").
// A truncated answer such as "ok:1" fails this check and is discarded.
func fullResponse(req, resp []byte) bool {
	if len(resp) < len(req)+1 {
		return false
	}
	if bytes.IndexByte(resp, ':') < 0 {
		return false
	}
	return bytes.HasSuffix(resp, req)
}

// deduper keys orders by their exact bytes so a new order that merely shares a
// prefix with a previous one is not swallowed.
type deduper struct {
	keys map[string]struct{}
}

func newDeduper() *deduper {
	return &deduper{keys: map[string]struct{}{}}
}

func (d *deduper) isSeen(req []byte) bool {
	_, ok := d.keys[string(req)]
	return ok
}

func (d *deduper) mark(req []byte) {
	d.keys[string(req)] = struct{}{}
}

// ledger collects completed responses in posting order, which follows the
// original request order of the surviving unique orders.
type ledger struct {
	out [][]byte
}

func newLedger() *ledger {
	return &ledger{}
}

func (l *ledger) post(resp []byte) {
	l.out = append(l.out, resp)
}

func (l *ledger) items() [][]byte {
	return l.out
}

func DecodeSeq(b []byte) int {
	if i := bytes.IndexByte(b, ':'); i >= 0 {
		n := 0
		for _, c := range b[:i] {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			}
		}
		return n
	}
	return 0
}
