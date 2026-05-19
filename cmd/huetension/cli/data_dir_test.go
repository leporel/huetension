package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveDataDirFlagPrecedence(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveDataDir(dir)
	if err != nil {
		t.Fatalf("flag: %v", err)
	}
	if got != dir {
		t.Errorf("flag: got %q, want %q", got, dir)
	}
}

func TestResolveDataDirFlagMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	_, err := resolveDataDir(missing)
	if err == nil || !strings.Contains(err.Error(), "--data-dir") {
		t.Errorf("expected --data-dir error for missing dir, got %v", err)
	}
}

func TestResolveDataDirEnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(dataDirEnvVar, dir)
	got, err := resolveDataDir("")
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	if got != dir {
		t.Errorf("env: got %q, want %q", got, dir)
	}
}

func TestResolveDataDirAutoDiscoveryUsesExistingUserDir(t *testing.T) {
	t.Setenv(dataDirEnvVar, "")
	home := setUserHome(t, t.TempDir())
	want := filepath.Join(home, ".config", "huetension")
	if err := os.MkdirAll(want, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got, err := resolveDataDir("")
	if err != nil {
		t.Fatalf("auto: %v", err)
	}
	assertSamePath(t, got, want)
}

func TestResolveDataDirAutoDiscoveryCreatesUserDirWhenMissing(t *testing.T) {
	t.Setenv(dataDirEnvVar, "")
	userDir := filepath.Join(t.TempDir(), "huetension")
	setDataDirResolverDeps(t, userDir)

	got, err := resolveDataDir("")
	if err != nil {
		t.Fatalf("auto: %v", err)
	}
	assertSamePath(t, got, userDir)
	if info, err := os.Stat(userDir); err != nil || !info.IsDir() {
		t.Fatalf("user dir was not created: info=%v err=%v", info, err)
	}
}

func setUserHome(t *testing.T, dir string) string {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("USERPROFILE", dir)
		t.Setenv("HOMEDRIVE", "")
		t.Setenv("HOMEPATH", "")
	default:
		t.Setenv("HOME", dir)
	}
	return dir
}

func assertSamePath(t *testing.T, got, want string) {
	t.Helper()
	wantResolved, _ := filepath.EvalSymlinks(want)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Errorf("path: got %q, want %q", got, want)
	}
}

func setDataDirResolverDeps(t *testing.T, userDir string) {
	t.Helper()
	origUser := defaultUserDataDirFunc
	origMkdirAll := mkdirAllFunc

	defaultUserDataDirFunc = func() string { return userDir }
	mkdirAllFunc = os.MkdirAll

	t.Cleanup(func() {
		defaultUserDataDirFunc = origUser
		mkdirAllFunc = origMkdirAll
	})
}

func TestLoadLibraryUsesEmbeddedWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	idx, savePath, err := loadLibrary(dir)
	if err != nil {
		t.Fatalf("loadLibrary: %v", err)
	}
	if idx.Len() == 0 {
		t.Fatal("expected embedded defaults when dir has no library.json")
	}
	// Even with no file present, a save path is offered so the first
	// save can bootstrap library.json inside the data dir.
	if want := filepath.Join(dir, libraryFilename); savePath != want {
		t.Errorf("savePath = %q, want %q", savePath, want)
	}
}

func TestLoadLibraryReadsExternalFromDataDir(t *testing.T) {
	dir := t.TempDir()
	body := `{"palettes":[{"id":"only","name":"Only","categories":["C"],"colors":["#112233"]}]}`
	if err := os.WriteFile(filepath.Join(dir, libraryFilename), []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	idx, savePath, err := loadLibrary(dir)
	if err != nil {
		t.Fatalf("loadLibrary: %v", err)
	}
	if want := filepath.Join(dir, libraryFilename); savePath != want {
		t.Errorf("savePath = %q, want %q", savePath, want)
	}
	if idx.Len() != 1 {
		t.Errorf("expected 1 palette from external file, got %d", idx.Len())
	}
	if _, ok := idx.Get("only"); !ok {
		t.Error("external palette 'only' missing")
	}
}
