package liquipedia

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

const oneTable = `<table class="wikitable collapsible"><tbody>
<tr><th>Patch</th><th>Release Date</th><th>Release Highlights</th></tr>
<tr><td><a href="/mobilelegends/Patch_2.1.95" title="Patch 2.1.95">Patch 2.1.95</a> (latest)</td>
    <td>August 4, 2026</td><td><ul><li>Revamped Hero Kaja</li></ul></td></tr>
<tr><td><a href="/mobilelegends/Patch_2.1.67a" title="Patch 2.1.67a">Patch 2.1.67a</a></td>
    <td>May 13, 2026</td><td>Hero adjustments</td></tr>
</tbody></table>`

func TestParsePatchesReadsVersionAndDate(t *testing.T) {
	got, err := ParsePatches(strings.NewReader(oneTable))
	if err != nil {
		t.Fatalf("ParsePatches: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 patches, got %d: %+v", len(got), got)
	}
	// "(latest)" is a display marker and must not leak into the version.
	if got[0].Version != "2.1.95" {
		t.Errorf("version = %q, want 2.1.95", got[0].Version)
	}
	want := time.Date(2026, time.August, 4, 0, 0, 0, 0, time.UTC)
	if !got[0].ReleaseDate.Equal(want) {
		t.Errorf("released = %v, want %v", got[0].ReleaseDate, want)
	}
	// Letter suffixes are real versions, not noise.
	if got[1].Version != "2.1.67a" {
		t.Errorf("version = %q, want 2.1.67a", got[1].Version)
	}
}

func TestParsePatchesSkipsHeaderAndAnchorRows(t *testing.T) {
	// The real page interleaves zero-height anchor rows between patch rows.
	html := `<table class="wikitable"><tbody>
<tr><th>Patch</th><th>Release Date</th></tr>
<tr style="height:0"><td colspan="3"><span id="Aug_2026"></span></td></tr>
<tr><td><a title="Patch 2.1.90">Patch 2.1.90</a></td><td>July 2, 2026</td><td>x</td></tr>
</tbody></table>`
	got, err := ParsePatches(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParsePatches: %v", err)
	}
	if len(got) != 1 || got[0].Version != "2.1.90" {
		t.Fatalf("want only the real patch row, got %+v", got)
	}
}

func TestParsePatchesIgnoresNonPatchRows(t *testing.T) {
	// A date-shaped cell in an unrelated table must not become a patch.
	html := `<table class="wikitable"><tbody>
<tr><td>Some Hero</td><td>January 1, 2020</td><td>released</td></tr>
<tr><td><a title="Patch 2.1.90">Patch 2.1.90</a></td><td>July 2, 2026</td><td>x</td></tr>
</tbody></table>`
	got, err := ParsePatches(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParsePatches: %v", err)
	}
	if len(got) != 1 || got[0].Version != "2.1.90" {
		t.Fatalf("want only the patch row, got %+v", got)
	}
}

func TestParsePatchesEmptyResultIsError(t *testing.T) {
	// A page that parses but yields nothing means the structure changed.
	_, err := ParsePatches(strings.NewReader(`<html><body><p>nothing here</p></body></html>`))
	if err == nil {
		t.Fatal("want an error when no patches are found")
	}
}

func TestParsePatchesAgainstRealPage(t *testing.T) {
	f, err := os.Open("testdata/patches.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got, err := ParsePatches(f)
	if err != nil {
		t.Fatalf("ParsePatches: %v", err)
	}
	if len(got) < 70 {
		t.Fatalf("want at least 70 patches from the real page, got %d", len(got))
	}

	// Release date is the natural key, not version: patches before 2025 reused
	// a version number across successive updates (1.9.42 ships five times).
	seen := map[string]bool{}
	var prev time.Time
	for i, p := range got {
		if p.Version == "" {
			t.Fatalf("patch %d has empty version", i)
		}
		day := p.ReleaseDate.Format("2006-01-02")
		if seen[day] {
			t.Errorf("duplicate release date %s (version %q)", day, p.Version)
		}
		seen[day] = true
		if p.ReleaseDate.IsZero() {
			t.Errorf("patch %q has no release date", p.Version)
		}
		if i > 0 && p.ReleaseDate.After(prev) {
			t.Errorf("patches not newest-first at %d: %v after %v", i, p.ReleaseDate, prev)
		}
		prev = p.ReleaseDate
	}

	if got[0].Version != "2.1.95" {
		t.Errorf("newest patch = %q, want 2.1.95", got[0].Version)
	}
}

func TestParseErrorDistinguishesStructureFromOddBody(t *testing.T) {
	// A real page with tables but no patch rows: the structure genuinely moved.
	structural := `<table class="wikitable"><tbody>
<tr><td>Hero</td><td>Role</td></tr>
<tr><td>Fanny</td><td>Assassin</td></tr></tbody></table>`
	_, err := ParsePatches(strings.NewReader(structural))
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ParseError, got %T: %v", err, err)
	}
	if pe.Tables == 0 {
		t.Errorf("want tables counted, got %d", pe.Tables)
	}
	if !strings.Contains(pe.Error(), "structure") {
		t.Errorf("a real page with tables should read as a structure change: %q", pe.Error())
	}

	// A short body with no tables: an error page or a throttle, not a redesign.
	_, err = ParsePatches(strings.NewReader(`<p>Too many requests</p>`))
	if !errors.As(err, &pe) {
		t.Fatalf("want *ParseError, got %T", err)
	}
	if pe.Tables != 0 {
		t.Errorf("want 0 tables, got %d", pe.Tables)
	}
	if strings.Contains(pe.Error(), "structure changed") {
		t.Errorf("a table-less body must not be diagnosed as a structure change: %q", pe.Error())
	}
	if !strings.Contains(pe.Error(), "unexpected") {
		t.Errorf("want an unexpected-body diagnosis, got %q", pe.Error())
	}
}

func TestParsePatchesReadsHighlightsAsList(t *testing.T) {
	html := `<table class="wikitable"><tbody>
<tr><th>Patch</th><th>Release Date</th><th>Release Highlights</th></tr>
<tr><td><a title="Patch 2.1.95">Patch 2.1.95</a> (latest)</td>
    <td>August 4, 2026</td>
    <td><ul><li>Revamped Hero  <a>Kaja</a>, Nazar King</li><li>Hero Adjustments</li></ul></td></tr>
</tbody></table>`
	got, err := ParsePatches(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParsePatches: %v", err)
	}
	want := []string{"Revamped Hero Kaja, Nazar King", "Hero Adjustments"}
	if len(got[0].Highlights) != len(want) {
		t.Fatalf("got %d highlights %q, want %d", len(got[0].Highlights), got[0].Highlights, len(want))
	}
	for i := range want {
		// Nested markup leaves doubled spaces in the raw text; entries are
		// stored as a reader would see them.
		if got[0].Highlights[i] != want[i] {
			t.Errorf("highlight %d = %q, want %q", i, got[0].Highlights[i], want[i])
		}
	}
}

func TestParsePatchesHighlightsFallBackToPlainCell(t *testing.T) {
	html := `<table class="wikitable"><tbody>
<tr><td><a title="Patch 2.1.67a">Patch 2.1.67a</a></td><td>May 13, 2026</td><td>Hero adjustments</td></tr>
</tbody></table>`
	got, err := ParsePatches(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParsePatches: %v", err)
	}
	if len(got[0].Highlights) != 1 || got[0].Highlights[0] != "Hero adjustments" {
		t.Errorf("want one plain highlight, got %q", got[0].Highlights)
	}
}

func TestParsePatchesMissingHighlightsCellIsNotAnError(t *testing.T) {
	// A two-column row still yields a patch: the date is what the calendar needs.
	html := `<table class="wikitable"><tbody>
<tr><td><a title="Patch 2.1.90">Patch 2.1.90</a></td><td>July 2, 2026</td></tr>
</tbody></table>`
	got, err := ParsePatches(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParsePatches: %v", err)
	}
	if got[0].Version != "2.1.90" {
		t.Fatalf("version = %q", got[0].Version)
	}
	if len(got[0].Highlights) != 0 {
		t.Errorf("want no highlights, got %q", got[0].Highlights)
	}
}

func TestParsePatchesRealPageCarriesHighlights(t *testing.T) {
	f, err := os.Open("testdata/patches.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := ParsePatches(f)
	if err != nil {
		t.Fatal(err)
	}
	withHighlights := 0
	for _, p := range got {
		for _, h := range p.Highlights {
			if strings.Contains(h, "  ") {
				t.Fatalf("patch %s highlight not whitespace-normalised: %q", p.Version, h)
			}
		}
		if len(p.Highlights) > 0 {
			withHighlights++
		}
	}
	// The real page carries a bullet list on the overwhelming majority of rows.
	if withHighlights < len(got)/2 {
		t.Errorf("only %d of %d patches carry highlights", withHighlights, len(got))
	}
	t.Logf("%d of %d patches carry highlights; newest %s: %q",
		withHighlights, len(got), got[0].Version, got[0].Highlights)
}
