package shelf

import (
	"errors"
	"image"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/servers/servertest"
)

// coverModified is when series s1 last changed, in the cover tests.
var coverModified = time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

// fetchTestCover fetches the cover of series s1 into new CoverFiles, fitting it into 140x210.
func fetchTestCover(t *testing.T) CoverFiles {
	t.Helper()
	covers := CoverFiles{Dir: t.TempDir()}
	if err := covers.Fetch(t.Context(), newTestServer(t), SeriesCover, "s1", 140, 210, coverModified); err != nil {
		t.Fatal(err)
	}
	return covers
}

func TestCoverMissingIsStale(t *testing.T) {
	if covers := (CoverFiles{Dir: t.TempDir()}); !covers.Stale(SeriesCover, "s1", time.Time{}) {
		t.Error("a missing cover is not stale")
	}
}

func TestCoverFetchAndLoad(t *testing.T) {
	cover, err := fetchTestCover(t).Load(SeriesCover, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if cover.Bounds() != image.Rect(0, 0, 140, 210) {
		t.Fatalf("size %v, want 140x210", cover.Bounds())
	}
}

func TestCoverStale(t *testing.T) {
	covers := fetchTestCover(t)
	if covers.Stale(SeriesCover, "s1", coverModified) {
		t.Error("a cover fetched at its series' date is stale")
	}
	if covers.Stale(SeriesCover, "s1", time.Time{}) {
		t.Error("a cover is stale when its series' date is unknown")
	}
	if !covers.Stale(SeriesCover, "s1", coverModified.Add(time.Hour)) {
		t.Error("a cover older than its series is not stale")
	}
}

func TestCoverKindsApart(t *testing.T) {
	if !fetchTestCover(t).Stale(BookCover, "s1", time.Time{}) {
		t.Error("a series cover is taken for a book's")
	}
}

func TestCoverFetchMissing(t *testing.T) {
	covers := CoverFiles{Dir: t.TempDir()}
	err := covers.Fetch(t.Context(), newTestServer(t), BookCover, "b1", 140, 210, coverModified)
	if !errors.Is(err, servertest.ErrNotFound) {
		t.Fatalf("error %v, want servertest.ErrNotFound", err)
	}
}
