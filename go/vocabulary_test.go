package lexicon

import (
	"path/filepath"
	"testing"
)

func testdataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "tests", "fixtures")
}

func TestLoadVocabularyFile(t *testing.T) {
	v, err := LoadVocabularyFile(filepath.Join(testdataDir(t), "vocabularies-test.yaml"))
	if err != nil {
		t.Fatalf("LoadVocabularyFile: %v", err)
	}

	realms, ok := v.Group("realms", "test_realm")
	if !ok {
		t.Fatal("expected realms.test_realm to exist")
	}
	if got, want := len(realms), 3; got != want {
		t.Errorf("realms.test_realm len = %d, want %d", got, want)
	}
	if got, want := realms[0], "alpha"; got != want {
		t.Errorf("realms.test_realm[0] = %q, want %q", got, want)
	}

	adj, ok := v.Group("adjectives", "any")
	if !ok || len(adj) != 2 {
		t.Errorf("adjectives.any missing or wrong length")
	}
}

func TestLoadVocabularyFile_Missing(t *testing.T) {
	_, err := LoadVocabularyFile("/no/such/file.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
