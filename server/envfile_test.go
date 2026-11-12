package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFiles(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, ".env")
	local := filepath.Join(dir, ".env.local")
	os.WriteFile(base, []byte(`# comment
INSPECT_TEST_A=from-env
INSPECT_TEST_B="quoted value"
export INSPECT_TEST_C=plain # trailing comment
INSPECT_TEST_D=from-env
`), 0o600)
	os.WriteFile(local, []byte("INSPECT_TEST_A=from-local\nINSPECT_TEST_E=\n"), 0o600)

	t.Setenv("INSPECT_TEST_D", "from-process")
	for _, k := range []string{"INSPECT_TEST_A", "INSPECT_TEST_B", "INSPECT_TEST_C", "INSPECT_TEST_E"} {
		t.Setenv(k, "") // registers cleanup
		os.Unsetenv(k)
	}

	loaded, err := loadEnvFiles(base, local, filepath.Join(dir, "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %v, want both files", loaded)
	}
	want := map[string]string{
		"INSPECT_TEST_A": "from-local",   // .env.local overrides .env
		"INSPECT_TEST_B": "quoted value", // quotes stripped
		"INSPECT_TEST_C": "plain",        // export prefix and trailing comment
		"INSPECT_TEST_D": "from-process", // real environment wins
		"INSPECT_TEST_E": "",
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestLoadEnvFilesRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(path, []byte("NOT A PAIR\n"), 0o600)
	if _, err := loadEnvFiles(path); err == nil {
		t.Fatal("expected an error for a line without '='")
	}
}
