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
	var cells []*html.Node
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "td" {
			cells = append(cells, c)
		}
	}
	if len(cells) < 2 {
		return domain.Patch{}, false
	}
	m := patchCell.FindStringSubmatch(strings.TrimSpace(textOf(cells[0])))
	if m == nil {
		return domain.Patch{}, false
	}
	released, err := time.Parse(releaseDateLayout, strings.TrimSpace(textOf(cells[1])))
	if err != nil {
		return domain.Patch{}, false
	}
	p := domain.Patch{Version: m[1], ReleaseDate: released}
	// The highlights column is presentational: a row without it is still a
	// patch, and the calendar's job is the date.
	if len(cells) > 2 {
		p.Highlights = highlightsOf(cells[2])
	}
	return p, true
}

// highlightsOf reads a release-highlights cell. The page renders it as a bullet
// list, so each item becomes one entry; a cell carrying plain text yields that
// text as a single entry, and an empty cell yields nothing.
func highlightsOf(td *html.Node) []string {
	var out []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" {
			// Nested lists collapse into their parent item rather than
			// producing an entry that repeats its parent's text.
			if s := normalizeSpace(textOf(n)); s != "" {
				out = append(out, s)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(td)
	if len(out) == 0 {
		if s := normalizeSpace(textOf(td)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// normalizeSpace collapses the runs of whitespace that nested markup leaves
// behind, so an entry reads the way the page displays it.
func normalizeSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

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
