package v1

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestHostKeyMatchesFingerprint(t *testing.T) {
	key := testPublicKey(t)
	other := testPublicKey(t)
	fp := ssh.FingerprintSHA256(key)

	for name, wanted := range map[string]string{
		"with prefix":    fp,
		"without prefix": strings.TrimPrefix(fp, "SHA256:"),
		"padded":         "  " + fp + " ",
	} {
		if !hostKeyMatchesFingerprint([]ssh.PublicKey{other, key}, wanted) {
			t.Errorf("%s: expected match", name)
		}
	}
	if hostKeyMatchesFingerprint([]ssh.PublicKey{other}, fp) {
		t.Error("unexpected match for a different key")
	}
	if hostKeyMatchesFingerprint([]ssh.PublicKey{key}, "") {
		t.Error("empty fingerprint must not match")
	}
}

func TestKnownHostsContent(t *testing.T) {
	key := testPublicKey(t)

	if got := knownHostsContent("nas.local", "22", []ssh.PublicKey{key}); !strings.HasPrefix(got, "nas.local ssh-ed25519 ") {
		t.Errorf("port 22 line = %q", got)
	}
	if got := knownHostsContent("nas.local", "2222", []ssh.PublicKey{key}); !strings.HasPrefix(got, "[nas.local]:2222 ssh-ed25519 ") {
		t.Errorf("custom port line = %q", got)
	}
}

func TestValidateBackupSourcePath(t *testing.T) {
	for _, bad := range []string{"", "/", "/etc", "/DATA/../etc", "/DATAX", "relative/path", "/root", "/var/lib/casaos"} {
		if _, err := validateBackupSourcePath(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
	for _, good := range []string{"/DATA", "/DATA/Documents", "/DATA/a/../b", "/mnt/nas"} {
		if _, err := validateBackupSourcePath(good); err != nil {
			t.Errorf("%q should be accepted: %v", good, err)
		}
	}
}
