package audit

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
)

// PEM block types for the keypair. The format is deliberately minimal: the raw
// Ed25519 seed (private) and raw public key bytes, wrapped in PEM. This avoids
// crypto/x509 (and its cgo path on macOS), keeping the package's trust chain to
// crypto/ed25519, crypto/sha256, crypto/rand, and encoding/pem alone.
const (
	privPEMType = "GOLOGX ED25519 PRIVATE KEY"
	pubPEMType  = "GOLOGX ED25519 PUBLIC KEY"
)

// GenerateKey returns a fresh Ed25519 keypair from crypto/rand. The private key
// signs entries as they are written; the public key verifies them later,
// typically on a different machine that never holds the private key.
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// MarshalPrivateKeyPEM encodes priv's 32-byte seed as a PEM block. Write it with
// 0600 permissions and keep it off the host whose logs it signs: anyone holding
// it can forge entries.
func MarshalPrivateKeyPEM(priv ed25519.PrivateKey) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("audit: private key is %d bytes, want %d", len(priv), ed25519.PrivateKeySize)
	}
	return pem.EncodeToMemory(&pem.Block{Type: privPEMType, Bytes: priv.Seed()}), nil
}

// MarshalPublicKeyPEM encodes pub as a PEM block. This is the file you hand to
// whoever needs to verify the log.
func MarshalPublicKeyPEM(pub ed25519.PublicKey) ([]byte, error) {
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("audit: public key is %d bytes, want %d", len(pub), ed25519.PublicKeySize)
	}
	return pem.EncodeToMemory(&pem.Block{Type: pubPEMType, Bytes: pub}), nil
}

// errNoPEM is returned when the input contains no PEM block at all.
var errNoPEM = errors.New("audit: no PEM block found")

// ParsePrivateKeyPEM parses a private key PEM block produced by
// MarshalPrivateKeyPEM and reconstructs the full Ed25519 private key.
func ParsePrivateKeyPEM(data []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errNoPEM
	}
	if block.Type != privPEMType {
		return nil, fmt.Errorf("audit: unexpected PEM type %q, want %q", block.Type, privPEMType)
	}
	if len(block.Bytes) != ed25519.SeedSize {
		return nil, fmt.Errorf("audit: private seed is %d bytes, want %d", len(block.Bytes), ed25519.SeedSize)
	}
	return ed25519.NewKeyFromSeed(block.Bytes), nil
}

// ParsePublicKeyPEM parses a public key PEM block produced by
// MarshalPublicKeyPEM.
func ParsePublicKeyPEM(data []byte) (ed25519.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errNoPEM
	}
	if block.Type != pubPEMType {
		return nil, fmt.Errorf("audit: unexpected PEM type %q, want %q", block.Type, pubPEMType)
	}
	if len(block.Bytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("audit: public key is %d bytes, want %d", len(block.Bytes), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(block.Bytes), nil
}
