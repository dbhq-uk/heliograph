package seal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The identity file, and the reason it lives here rather than in a command.
//
// Two binaries read and write it. `heliograph-seal` is what the STATION runs -
// it is the one exception to nothing being installed on the far side - and
// `heliograph` is what the control node runs, and neither can be told to
// re-derive a key. A second copy of this struct in a second package is a
// format that drifts silently, and the symptom is a station that cannot verify
// a request on a machine nobody can log into.
//
// JSON with the public half written alongside the secret, and that redundancy
// is deliberate: it lets a peer file and an identity file be the same shape, so
// nobody has to remember which they were handed.
type storedIdentity struct {
	Version int    `json:"version"`
	Secret  string `json:"secret"`
	Public  string `json:"public"`
}

// WriteIdentityFile writes a new identity at mode 600, refusing to replace one.
//
// REFUSING IS THE POINT. An identity file replaced by accident is a station
// that can no longer be talked to and a control that can no longer read its
// logs, on a machine nobody can reach - and there is no recovery except
// planting a new station. A file that already exists is far more likely to be
// somebody's working key than a leftover.
func WriteIdentityFile(path string, id *Identity) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists: refusing to replace an identity, because everything sealed to it becomes unreadable", path)
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(storedIdentity{
		Version: 1, Secret: id.Encode(), Public: id.Public().Encode(),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

// LoadIdentityFile reads the secret half back.
func LoadIdentityFile(path string) (*Identity, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s storedIdentity
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s is not an identity file: %w", path, err)
	}
	if s.Secret == "" {
		return nil, fmt.Errorf("%s holds a public identity and no secret: it is a PEER file, not an identity", path)
	}
	return DecodeIdentity(s.Secret)
}

// LoadPeerFile reads the other side's public identity.
//
// A peer file may be a bare public identity on one line, or the JSON an
// identity file happens to be. Accepting both means nobody has to remember
// which they were sent, and getting it wrong is a station that starts and then
// fails every verification.
//
// A JSON identity file is accepted for its PUBLIC half only. That is the shape
// of an accident worth surviving: somebody sends the whole file rather than the
// one line, and the alternative to reading its public half is refusing a peer
// that is sitting right there.
func LoadPeerFile(path string) (PublicIdentity, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return PublicIdentity{}, err
	}
	txt := strings.TrimSpace(string(b))
	if strings.HasPrefix(txt, "{") {
		var s storedIdentity
		if err := json.Unmarshal([]byte(txt), &s); err == nil && s.Public != "" {
			return DecodePublic(s.Public)
		}
	}
	return DecodePublic(txt)
}

// PublicOf reads an identity file and returns only what may be sent to the
// other side. Named so that a call site reads as what it is.
func PublicOf(path string) (PublicIdentity, error) {
	id, err := LoadIdentityFile(path)
	if err != nil {
		return PublicIdentity{}, err
	}
	return id.Public(), nil
}
