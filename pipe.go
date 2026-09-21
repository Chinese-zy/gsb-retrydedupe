package retrydedupe

import "bytes"

// Handle is the public entry: retry + dedupe + ledger in one function.
func Handle(reqs [][]byte, down func([]byte) ([]byte, error)) ([][]byte, error) {
	seen := map[string]bool{}
	var out [][]byte
	for _, req := range reqs {
		key := string(req)
		// BUG: dedupe by full bytes so "alike" new orders with shared prefix can be swallowed if truncated compare
		if len(key) > 2 {
			key = key[:2]
		}
		if seen[key] {
			continue
		}
		var last []byte
		var err error
		for try := 0; try < 3; try++ {
			last, err = down(req)
			if err == nil {
				break
			}
			// BUG: on jitter, re-exec even if previous try already produced a full ledger entry
		}
		if err != nil {
			// BUG: half response still advances and skips later good seq
			if last != nil {
				out = append(out, last)
			}
			continue
		}
		seen[key] = true
		out = append(out, last)
	}
	return out, nil
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
