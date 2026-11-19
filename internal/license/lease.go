// The lease format, copied from kashi-license/lease (see that package for the
// wire contract). testdata/golden.json is the server's own fixture; keep both
// in step. A lease is the signed permission a till needs to run.
//
// The license server signs a Lease with its Ed25519 private key; the till only
// holds the public key, so it can check a lease but never make one. A lease
// says which install and machine it is for, whether the till may run ("on") or
// must lock ("off"), and until when it stays good without a new one. A till
// that stays offline past ValidUntil refuses to start.
//
// The signature covers the exact JSON bytes the server produced (Signed.Payload),
// so neither side has to agree on a canonical JSON form. kashi-pos keeps a copy
// of this file in internal/license; the golden fixture in testdata is checked
// by both repos, so a change to the format on one side fails a test on the other.
package license

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"
)

// Version is the lease format version. A till refuses a lease of another version.
const Version = 1

const (
	StatusOn  = "on"
	StatusOff = "off"
)

// Lease is what the server signs.
type Lease struct {
	V                 int       `json:"v"`
	InstallID         string    `json:"installId"`
	Fingerprint       string    `json:"fingerprint"`
	Status            string    `json:"status"`
	LicensedTo        string    `json:"licensedTo"`
	IssuedAt          time.Time `json:"issuedAt"`
	ValidUntil        time.Time `json:"validUntil"`
	CheckEverySeconds int       `json:"checkEverySeconds"`
}

// Signed is a lease as it travels and as the till stores it: the server's JSON
// bytes and the signature over them, both base64 (std, padded).
type Signed struct {
	Payload   string `json:"p"`
	Signature string `json:"s"`
}

// Sign serialises l and signs the bytes.
func Sign(priv ed25519.PrivateKey, l Lease) (Signed, error) {
	raw, err := json.Marshal(l)
	if err != nil {
		return Signed{}, err
	}
	return Signed{
		Payload:   base64.StdEncoding.EncodeToString(raw),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, raw)),
	}, nil
}

var (
	ErrBadSignature = errors.New("lease signature does not verify")
	ErrBadVersion   = errors.New("lease version not supported")
)

// Verify checks the signature and returns the lease. It does not judge the
// lease's dates or status; the caller does, against its own clock.
func Verify(pub ed25519.PublicKey, s Signed) (Lease, error) {
	raw, err := base64.StdEncoding.DecodeString(s.Payload)
	if err != nil {
		return Lease{}, fmt.Errorf("lease payload: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(s.Signature)
	if err != nil {
		return Lease{}, fmt.Errorf("lease signature: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, raw, sig) {
		return Lease{}, ErrBadSignature
	}
	var l Lease
	if err := json.Unmarshal(raw, &l); err != nil {
		return Lease{}, fmt.Errorf("lease json: %w", err)
	}
	if l.V != Version {
		return Lease{}, ErrBadVersion
	}
	return l, nil
}

// ParsePublicKey reads a PEM "PUBLIC KEY" (PKIX) Ed25519 key.
func ParsePublicKey(pemText string) (ed25519.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, errors.New("public key: no PEM block")
	}
	k, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("public key: %w", err)
	}
	pub, ok := k.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("public key: not Ed25519")
	}
	return pub, nil
}

// ParsePrivateKey reads a PEM "PRIVATE KEY" (PKCS8) Ed25519 key.
func ParsePrivateKey(pemText string) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, errors.New("private key: no PEM block")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("private key: %w", err)
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("private key: not Ed25519")
	}
	return priv, nil
}

// EncodeKeys returns PEM text for a key pair (as cmd/genkey prints it).
func EncodeKeys(pub ed25519.PublicKey, priv ed25519.PrivateKey) (pubPEM, privPEM string, err error) {
	pb, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", "", err
	}
	sb, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pb})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: sb})), nil
}
