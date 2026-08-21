// Package liquipedia reads the MLBB patch calendar from Liquipedia's MediaWiki
// API. It is the only source of patch release dates: Moonton's stats endpoint
// carries no patch stamp of any kind, so patch context is joined by date rather
// than read from the statistics themselves.
package liquipedia

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/yeremi777/mlbb-analyzer-service/internal/domain"
)

// patchCell matches the first cell of a patch row: "Patch 2.1.95", optionally
// followed by a display marker such as "(latest)". The version may carry a
// letter suffix, as in 2.1.67a, which is a distinct release.
var patchCell = regexp.MustCompile(`^Patch\s+([0-9][0-9a-z.]*)`)

const releaseDateLayout = "January 2, 2006"

// ParsePatches reads the rendered Portal:Patches HTML and returns every patch
// it can identify, newest first, as the page orders them.
//
// Rows are read cell by cell rather than by pattern-matching the document:
// the page carries date-shaped text in cells that are not release dates, so
// pairing by position within a row is the only reliable read.
func ParsePatches(r io.Reader) ([]domain.Patch, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read patches body: %w", err)
	}
	doc, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse patches html: %w", err)
	}

	var patches []domain.Patch
	tables := 0
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			tables++
		}
		if n.Type == html.ElementNode && n.Data == "tr" {
			if p, ok := patchFromRow(n); ok {
				patches = append(patches, p)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if len(patches) == 0 {
		return nil, &ParseError{Bytes: len(raw), Tables: tables}
	}
	return patches, nil
}

// ParseError reports a body that fetched cleanly but carried no patches.
//
// The table count separates two causes that need opposite responses: a full
// page whose tables no longer hold patch rows means Portal:Patches was
// redesigned and the parser needs work, while a body with no tables at all is
// an error or throttle page, which is worth retrying.
type ParseError struct {
	Bytes  int
	Tables int
}

func (e *ParseError) Error() string {
	if e.Tables > 0 {
		return fmt.Sprintf("no patches found in %d bytes across %d tables: Portal:Patches structure changed",
			e.Bytes, e.Tables)
	}
	return fmt.Sprintf("no patches found in %d bytes with no tables: unexpected body (error or throttle page)",
		e.Bytes)
}

// Retryable reports whether refetching could plausibly succeed. A redesigned
// page parses the same way every time; an error page may not.
func (e *ParseError) Retryable() bool { return e.Tables == 0 }

// patchFromRow reads one table row. A row qualifies only when its first cell
// names a patch and its second cell parses as a release date; header rows,
// anchor rows and unrelated tables all fail one of those and are skipped.
func patchFromRow(tr *html.Node) (domain.Patch, bool) {
	var cells []string
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "td" {
			cells = append(cells, strings.TrimSpace(textOf(c)))
		}
	}
	if len(cells) < 2 {
		return domain.Patch{}, false
	}
	m := patchCell.FindStringSubmatch(cells[0])
	if m == nil {
		return domain.Patch{}, false
	}
	released, err := time.Parse(releaseDateLayout, cells[1])
	if err != nil {
		return domain.Patch{}, false
	}
	return domain.Patch{Version: m[1], ReleaseDate: released}, true
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}
