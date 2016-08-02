package mtree

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdate(t *testing.T) {
	content := []byte("I know half of you half as well as I ought to")
	dir, err := ioutil.TempDir("", "test-check-keywords")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up

	tmpfn := filepath.Join(dir, "tmpfile")
	if err := ioutil.WriteFile(tmpfn, content, 0666); err != nil {
		t.Fatal(err)
	}

	// Walk this tempdir
	dh, err := Walk(dir, nil, append(DefaultKeywords, "sha1"))
	if err != nil {
		t.Fatal(err)
	}

	// Touch a file, so the mtime changes.
	now := time.Now()
	if err := os.Chtimes(tmpfn, now, now); err != nil {
		t.Fatal(err)
	}

	// Check for sanity. This ought to have failures
	res, err := Check(dir, dh, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) == 0 {
		t.Error("expected failures (like mtimes), but got none")
	}

	res, err = Update(dir, dh, DefaultUpdateKeywords)
	if err != nil {
		t.Error(err)
	}
	if len(res.Failures) > 0 {
		t.Errorf("%#v", res.Failures)
	}

	// Now check that we're sane again
	res, err = Check(dir, dh, nil)
	if err != nil {
		t.Fatal(err)
	}
	// should have no failures now
	if len(res.Failures) > 0 {
		t.Errorf("%#v", res.Failures)
	}

}
