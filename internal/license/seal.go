package license

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

// The license file (install secret, database key, lease) is sealed twice:
//
//   - with AES-GCM under a key derived from the machine fingerprint, so it
//     only opens on the PC it was made on;
//   - on Windows, also with DPAPI for the local machine (platformSeal), so the
//     file can't be opened by copying it plus a faked fingerprint elsewhere.
//
// Neither makes the file safe from someone who runs code on this very PC;
// they keep a copied file (or a copied database) useless on another one.

var ErrUnseal = errors.New("license file does not open on this machine")

const sealInfo = "kashi-pos-license-state-v1|"

func fingerprintKey(fingerprint string) []byte {
	sum := sha256.Sum256([]byte(sealInfo + fingerprint))
	return sum[:]
}

func seal(fingerprint string, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(fingerprintKey(fingerprint))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	inner := gcm.Seal(nonce, nonce, plain, []byte(sealInfo))
	return platformSeal(inner)
}

func unseal(fingerprint string, sealed []byte) ([]byte, error) {
	inner, err := platformUnseal(sealed)
	if err != nil {
		return nil, ErrUnseal
	}
	block, err := aes.NewCipher(fingerprintKey(fingerprint))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(inner) < gcm.NonceSize() {
		return nil, ErrUnseal
	}
	plain, err := gcm.Open(nil, inner[:gcm.NonceSize()], inner[gcm.NonceSize():], []byte(sealInfo))
	if err != nil {
		return nil, ErrUnseal
	}
	return plain, nil
}
