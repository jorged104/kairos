package httpx

import (
	"net/http"
	"strconv"
)

// Page is a 1-based page request.
type Page struct {
	Number int
	Size   int
}

func (p Page) Offset() int { return (p.Number - 1) * p.Size }

// ParsePage reads ?page and ?page_size; ok is false if they are not positive
// integers or the size exceeds maxSize.
func ParsePage(r *http.Request, defSize, maxSize int) (Page, bool) {
	p := Page{Number: 1, Size: defSize}
	q := r.URL.Query()
	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return p, false
		}
		p.Number = n
	}
	if v := q.Get("page_size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxSize {
			return p, false
		}
		p.Size = n
	}
	return p, true
}
