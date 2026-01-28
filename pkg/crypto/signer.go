package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/zen-systems/flowgate/pkg/schema"
)

// Signer handles signing of artifacts.
type Signer struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
	KeyID      string
}

// NewSigner creates or loads a signer.
func NewSigner(keyID string) (*Signer, error) {
	// For simplicity, we'll try to load key from ~/.flowgate/keys/keyID.pem
	// If not exists, generate one.
	home, _ := os.UserHomeDir()
	keyDir := filepath.Join(home, ".flowgate", "keys")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, err
	}

	keyPath := filepath.Join(keyDir, keyID+".key")

	var privateKey ed25519.PrivateKey

	data, err := os.ReadFile(keyPath)
	if err == nil {
		privateKey = ed25519.PrivateKey(data)
	} else {
		// Generate new key
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		privateKey = priv
		if err := os.WriteFile(keyPath, []byte(privateKey), 0600); err != nil {
			return nil, err
		}
	}

	return &Signer{
		PrivateKey: privateKey,
		PublicKey:  privateKey.Public().(ed25519.PublicKey),
		KeyID:      keyID,
	}, nil
}

// SignAttestation signs the attestation and attaches the signature.
func (s *Signer) SignAttestation(att *schema.AttestationV1) error {
	// Canonicalize content to sign (without signature field)
	// schema struct has Signature *Signature `json:"signature,omitempty"`
	// so if it's nil, it's omitted.
	att.Signature = nil

	data, err := json.Marshal(att)
	if err != nil {
		return err
	}

	sig := ed25519.Sign(s.PrivateKey, data)
	sigStr := base64.StdEncoding.EncodeToString(sig)

	att.Signature = &schema.Signature{
		Alg:      "ed25519",
		PubKeyID: s.KeyID,
		Sig:      sigStr,
	}

	return nil
}
