package v1

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func makeArchive(t *testing.T, entries map[string]string) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	return bytes.NewReader(buf.Bytes())
}

const goodManifest = `{"format":1,"created":"2026-10-29T00:00:00Z","casaos_version":"0.4.15","files":[]}`

func TestReadConfigArchiveAcceptsExpectedFiles(t *testing.T) {
	files, err := readConfigArchive(makeArchive(t, map[string]string{
		"manifest.json":         goodManifest,
		"config/casaos.conf":    "[common]\n",
		"data/backup_jobs.json": "[]",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("want 2 files, got %v", files)
	}
}

func TestReadConfigArchiveRejectsHostileArchives(t *testing.T) {
	cases := map[string]map[string]string{
		"path traversal":     {"manifest.json": goodManifest, "config/../../etc/passwd": "x"},
		"absolute path":      {"manifest.json": goodManifest, "/etc/passwd": "x"},
		"nested config dir":  {"manifest.json": goodManifest, "config/sub/casaos.conf": "x"},
		"key material":       {"manifest.json": goodManifest, "config/jwt-private.key.conf": "x"},
		"wrong extension":    {"manifest.json": goodManifest, "config/run.sh": "x"},
		"no manifest":        {"config/casaos.conf": "x"},
		"bad manifest":       {"manifest.json": `{"format":9}`, "config/casaos.conf": "x"},
		"invalid jobs":       {"manifest.json": goodManifest, "data/backup_jobs.json": "not json"},
		"only a manifest":    {"manifest.json": goodManifest},
		"unknown data entry": {"manifest.json": goodManifest, "data/casaos.db": "x"},
	}
	for name, entries := range cases {
		if _, err := readConfigArchive(makeArchive(t, entries)); err == nil {
			t.Errorf("%s: archive was accepted", name)
		}
	}
	if _, err := readConfigArchive(strings.NewReader("this is not gzip")); err == nil {
		t.Error("garbage was accepted")
	}
}

func TestIsConfigFileName(t *testing.T) {
	good := []string{"casaos.conf", "gateway.ini", "app-management.conf"}
	bad := []string{"", ".hidden.conf", "a/b.conf", `a\b.conf`, "secret.conf", "x.key", "run.sh", "private-key.ini"}
	for _, n := range good {
		if !isConfigFileName(n) {
			t.Errorf("%q should be accepted", n)
		}
	}
	for _, n := range bad {
		if isConfigFileName(n) {
			t.Errorf("%q should be refused", n)
		}
	}
}

func TestHealthHistoryTrimsAndSpaces(t *testing.T) {
	h := &healthHistory{samples: map[string][]HealthSample{}}
	base := time.Now().UnixMilli()
	h.record("/mnt/a", HealthSample{Time: base - 25*3600*1000, Healthy: true, LatencyMs: 5}) // older than the retention
	h.record("/mnt/a", HealthSample{Time: base, Healthy: true, LatencyMs: 10})
	h.record("/mnt/a", HealthSample{Time: base + 1000, Healthy: true, LatencyMs: 11}) // too close: ignored
	h.record("/mnt/a", HealthSample{Time: base + 60000, Healthy: false, LatencyMs: 0})

	got := h.since("/mnt/a", 0)["/mnt/a"]
	if len(got) != 2 || got[0].LatencyMs != 10 || got[1].Healthy {
		t.Fatalf("unexpected history: %+v", got)
	}
	if n := len(h.since("/mnt/a", base+30000)["/mnt/a"]); n != 1 {
		t.Fatalf("since filter: got %d", n)
	}
	h.forget(map[string]bool{})
	if len(h.since("", 0)) != 0 {
		t.Fatal("unmounted storage kept its history")
	}
}

func TestMatchesType(t *testing.T) {
	if !matchesType("image", "Photo.JPG", false) || matchesType("image", "a.mp4", false) || matchesType("image", "dir.png", true) {
		t.Error("image filter is wrong")
	}
	if !matchesType("folder", "x", true) || matchesType("folder", "x.txt", false) {
		t.Error("folder filter is wrong")
	}
	if !matchesType("", "anything", false) || matchesType("nonsense", "a.txt", false) {
		t.Error("any/unknown filter is wrong")
	}
}

func TestTreeSize(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755)
	os.WriteFile(filepath.Join(dir, "one"), make([]byte, 100), 0o644)
	os.WriteFile(filepath.Join(dir, "a", "two"), make([]byte, 50), 0o644)
	os.WriteFile(filepath.Join(dir, "a", "b", "three"), make([]byte, 25), 0o644)
	if got := treeSize(context.Background(), dir); got != 175 {
		t.Fatalf("want 175, got %d", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := treeSize(ctx, dir); got > 175 {
		t.Fatalf("cancelled walk returned %d", got)
	}
}

func TestShareLinkUsable(t *testing.T) {
	now := time.Now()
	link := &shareLink{ExpiresAt: now.Add(time.Hour).Unix(), MaxDownloads: 2, Downloads: 1}
	if !link.usable(now) {
		t.Error("fresh link should be usable")
	}
	link.Downloads = 2
	if link.usable(now) {
		t.Error("link at its download limit should not be usable")
	}
	link.Downloads, link.MaxDownloads = 99, 0
	if !link.usable(now) {
		t.Error("unlimited link should be usable")
	}
	if link.usable(now.Add(2 * time.Hour)) {
		t.Error("expired link should not be usable")
	}
}
