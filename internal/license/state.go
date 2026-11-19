package license

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State is what activation leaves on the PC, in one sealed file.
type State struct {
	InstallID     string `json:"installId"`
	InstallSecret string `json:"installSecret"`
	// DBKey is the 64-hex-character key the till's databases are encrypted
	// with. The server keeps a copy, so activating again on the same PC gives
	// the same key back.
	DBKey string `json:"dbKey"`
	Lease Signed `json:"lease"`
	// LastSeen is the latest wall-clock time this till has run at. A clock
	// set back before it can't stretch an old lease.
	LastSeen time.Time `json:"lastSeen"`
}

var ErrNoState = errors.New("not activated")

// StatePath is the license file, next to the branch registry; it belongs to
// the machine, not to one branch database.
func StatePath(dir string) string { return filepath.Join(dir, "license.dat") }

func LoadState(path, fingerprint string) (State, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, ErrNoState
	}
	if err != nil {
		return State{}, err
	}
	plain, err := unseal(fingerprint, raw)
	if err != nil {
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(plain, &st); err != nil {
		return State{}, fmt.Errorf("license file: %w", err)
	}
	if st.InstallID == "" || st.InstallSecret == "" || len(st.DBKey) != 64 {
		return State{}, errors.New("license file is incomplete")
	}
	return st, nil
}

// SaveState writes the file atomically (temp file, then rename), so a power
// cut mid-write leaves the old file, never a half one.
func SaveState(path, fingerprint string, st State) error {
	plain, err := json.Marshal(st)
	if err != nil {
		return err
	}
	sealed, err := seal(fingerprint, plain)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, sealed, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
