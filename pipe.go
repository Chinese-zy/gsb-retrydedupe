package retrydedupe

import "bytes"

// Handle is the public entry. Wire format is unchanged: requests go to
// down verbatim, completed entries come out verbatim in request order.
func Handle(reqs [][]byte, down func([]byte) ([]byte, error)) ([][]byte, error) {
	d := newDedupe()
	l := &ledger{}
	for _, req := range reqs {
		if d.seen(req) {
			continue
		}
		entry, ok := fetch(req, down)
		if !ok {
			// never committed: not marked seen, later duplicate may retry
			continue
		}
		d.mark(req)
		l.add(entry)
	}
	return l.entries, nil
}

const maxTries = 3

// fetch computes one order. It retries only when downstream gave nothing
// usable back; a full entry is accepted even when its call reported jitter,
// so an already-committed order is never executed twice. Half replies are
// dropped and never reach the ledger.
func fetch(req []byte, down func([]byte) ([]byte, error)) ([]byte, bool) {
	for try := 0; try < maxTries; try++ {
		resp, err := down(req)
		if err == nil {
			return resp, true
		}
		if isFullEntry(req, resp) {
			// jitter after the entry was committed: keep it, do not re-execute
			return resp, true
		}
		// half reply: drop it and try again
	}
	return nil, false
}

// isFullEntry reports whether resp carries the complete order back.
func isFullEntry(req, resp []byte) bool {
	return len(resp) > 0 && bytes.Contains(resp, req)
}

// dedupe tracks orders already computed, keyed by the full request bytes,
// so a new order that merely shares a prefix is never swallowed.
type dedupe struct {
	done map[string]struct{}
}

func newDedupe() *dedupe {
	return &dedupe{done: map[string]struct{}{}}
}

func (d *dedupe) seen(req []byte) bool {
	_, ok := d.done[string(req)]
	return ok
}

func (d *dedupe) mark(req []byte) {
	d.done[string(req)] = struct{}{}
}

// ledger keeps completed entries in their original request order.
type ledger struct {
	entries [][]byte
}

func (l *ledger) add(entry []byte) {
	l.entries = append(l.entries, entry)
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
