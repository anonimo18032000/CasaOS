package v1

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// rclone's sftp backend only verifies the server's identity when it is given a known_hosts file;
// without one it accepts any host key, so a man-in-the-middle could read the credentials. These
// helpers read the server's host keys up front and write a known_hosts file for rclone to enforce.
// If the user supplies the expected fingerprint it is checked against what the server presents;
// otherwise the key seen now is pinned (trust on first use), which still protects every later
// connection.

const hostKeyScanTimeout = 8 * time.Second

var errHostKeyCaptured = errors.New("host key captured")

var hostKeyAlgorithmsToScan = []string{
	ssh.KeyAlgoED25519,
	ssh.KeyAlgoECDSA256,
	ssh.KeyAlgoECDSA384,
	ssh.KeyAlgoECDSA521,
	ssh.KeyAlgoRSASHA512,
	ssh.KeyAlgoRSASHA256,
}

func sftpKnownHostsDir() string {
	return filepath.Join(constants.DefaultDataPath, "sftp_known_hosts")
}

// scanSSHHostKeys returns every distinct host key the server offers. The server may prefer a
// different key type than the one we pin, so each algorithm is tried separately and all keys found
// are trusted for that host.
func scanSSHHostKeys(host, port string) ([]ssh.PublicKey, error) {
	address := net.JoinHostPort(host, port)
	var keys []ssh.PublicKey
	seen := map[string]bool{}
	var lastErr error

	for i, algorithm := range hostKeyAlgorithmsToScan {
		conn, err := net.DialTimeout("tcp", address, hostKeyScanTimeout)
		if err != nil {
			if i == 0 {
				return nil, err
			}
			lastErr = err
			continue
		}

		var captured ssh.PublicKey
		config := &ssh.ClientConfig{
			User:              "casaos-hostkey-scan",
			HostKeyAlgorithms: []string{algorithm},
			Timeout:           hostKeyScanTimeout,
			HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
				captured = key
				return errHostKeyCaptured
			},
		}

		_ = conn.SetDeadline(time.Now().Add(hostKeyScanTimeout))
		_, _, _, err = ssh.NewClientConn(conn, address, config)
		_ = conn.Close()

		if captured == nil {
			if err != nil {
				lastErr = err
			}
			continue
		}

		id := string(captured.Marshal())
		if !seen[id] {
			seen[id] = true
			keys = append(keys, captured)
		}
	}

	if len(keys) == 0 {
		if lastErr == nil {
			lastErr = errors.New("server did not present a host key")
		}
		return nil, lastErr
	}
	return keys, nil
}

func describeHostKeys(keys []ssh.PublicKey) []string {
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, fmt.Sprintf("%s (%s)", ssh.FingerprintSHA256(key), key.Type()))
	}
	return out
}

// hostKeyMatchesFingerprint reports whether any of keys has the given fingerprint. The "SHA256:"
// prefix is optional.
func hostKeyMatchesFingerprint(keys []ssh.PublicKey, wanted string) bool {
	wanted = strings.TrimSpace(wanted)
	wanted = strings.TrimPrefix(wanted, "SHA256:")
	if wanted == "" {
		return false
	}
	for _, key := range keys {
		if strings.TrimPrefix(ssh.FingerprintSHA256(key), "SHA256:") == wanted {
			return true
		}
	}
	return false
}

func knownHostsContent(host, port string, keys []ssh.PublicKey) string {
	var b strings.Builder
	address := knownhosts.Normalize(net.JoinHostPort(host, port))
	for _, key := range keys {
		b.WriteString(knownhosts.Line([]string{address}, key))
		b.WriteString("\n")
	}
	return b.String()
}

// pinHostKeys scans the server, checks the optional expected fingerprint and writes a known_hosts
// file at path. It returns the fingerprints the server presented.
func pinHostKeys(host, port, expectedFingerprint, path string) ([]string, error) {
	keys, err := scanSSHHostKeys(host, port)
	if err != nil {
		return nil, fmt.Errorf("could not read the SSH host key of %s:%s: %w", host, port, err)
	}

	presented := describeHostKeys(keys)
	if strings.TrimSpace(expectedFingerprint) != "" && !hostKeyMatchesFingerprint(keys, expectedFingerprint) {
		return presented, fmt.Errorf("host key fingerprint mismatch: the server presented %s", strings.Join(presented, ", "))
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return presented, err
	}
	if err := os.WriteFile(path, []byte(knownHostsContent(host, port, keys)), 0o600); err != nil {
		return presented, err
	}
	return presented, nil
}
