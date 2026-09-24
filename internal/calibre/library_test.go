package calibre_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/calibre"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// mountTree mimics a TrueNAS share mounted at /calibre, e.g. red14/plex:
//
//	books/Clean Library/metadata.db   (the library we want)
//	books/Other Library/metadata.db
//	movies/
func mountTree(t *testing.T) string {
	t.Helper()
	mount := t.TempDir()
	makeLibraryAt(t, filepath.Join(mount, "books", "Clean Library"))
	makeLibraryAt(t, filepath.Join(mount, "books", "Other Library"))
	if err := os.MkdirAll(filepath.Join(mount, "movies"), 0o755); err != nil {
		t.Fatal(err)
	}
	return mount
}

func TestResolveRelStaysInsideMount(t *testing.T) {
	mount := mountTree(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(mount, "escape")); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"../", "../../etc", "/etc"} {
		abs, err := calibre.ResolveRel(mount, rel)
		// ".." is clamped to the mount root; absolute paths are re-rooted.
		if err == nil && abs != mount && abs != filepath.Join(mount, "etc") {
			t.Fatalf("%q resolved outside mount: %s", rel, abs)
		}
	}
	if _, err := calibre.ResolveRel(mount, "escape"); err != calibre.ErrOutsideMount {
		t.Fatalf("symlink escape not blocked: %v", err)
	}
}

func TestBrowseAndFind(t *testing.T) {
	mount := mountTree(t)
	l, err := calibre.Browse(mount, "books")
	if err != nil {
		t.Fatal(err)
	}
	if l.Path != "books" || l.Parent == nil || *l.Parent != "" || len(l.Folders) != 2 {
		t.Fatalf("unexpected listing %+v", l)
	}
	if l.Folders[0].Name != "Clean Library" || !l.Folders[0].HasLibrary || l.Folders[0].Path != "books/Clean Library" {
		t.Fatalf("unexpected folder %+v", l.Folders[0])
	}
	found, err := calibre.FindLibraries(mount, 5, 1000)
	if err != nil || len(found) != 2 {
		t.Fatalf("find: %v %+v", err, found)
	}
}

func TestSetLibraryResyncsFromNewFolder(t *testing.T) {
	mount := mountTree(t)
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	s := &calibre.Syncer{Store: st, Dir: mount}

	if s.Available() {
		t.Fatal("mount root has no metadata.db, should not be available")
	}
	if _, err := s.SetLibrary("movies"); err == nil {
		t.Fatal("expected error for folder without metadata.db")
	}
	rel, err := s.SetLibrary("books/Clean Library")
	if err != nil || rel != "books/Clean Library" {
		t.Fatalf("set library: %q %v", rel, err)
	}
	if _, err := s.Run(); err != nil {
		t.Fatal(err)
	}
	books, _, _ := st.ListBooks(store.BookFilter{Query: "Good Omens"}, nil)
	if len(books) != 1 {
		t.Fatalf("expected synced book, got %d", len(books))
	}
	_ = st.SaveAnalysis(books[0].ID, store.Analysis{Classification: "No Spice"})
	copies, _ := st.BookCopies(books[0].ID)
	want := filepath.Join(mount, "books/Clean Library/Terry Pratchett/Good Omens (2)/Good Omens - Terry Pratchett.epub")
	if copies[0].Path != want {
		t.Fatalf("path %s, want %s", copies[0].Path, want)
	}

	// Switching to another library with the same titles keeps analyses and
	// re-points file paths at the new folder.
	if _, err := s.SetLibrary("books/Other Library"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Run(); err != nil {
		t.Fatal(err)
	}
	b, err := st.BookByID(books[0].ID, nil)
	if err != nil || b.Classification == nil || *b.Classification != "No Spice" {
		t.Fatalf("analysis lost after switching library: %+v %v", b, err)
	}
	copies, _ = st.BookCopies(b.ID)
	if len(copies) != 1 || filepath.Dir(filepath.Dir(filepath.Dir(copies[0].Path))) != filepath.Join(mount, "books/Other Library") {
		t.Fatalf("copies not re-pointed: %+v", copies)
	}
}
