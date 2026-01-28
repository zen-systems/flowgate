package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	if att == nil {
		return fmt.Errorf("attestation required")
	}

	attCopy := *att
	attCopy.Signature = nil
	if err := attCopy.Validate(); err != nil {
		return err
	}

	data, err := json.Marshal(&attCopy)
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

// VerifyAttestationSignature verifies the attached signature against the payload.
func VerifyAttestationSignature(att *schema.AttestationV1) error {
	if att == nil {
		return fmt.Errorf("attestation required")
	}
	if att.Signature == nil {
		return fmt.Errorf("signature required")
	}
	if err := att.Signature.Validate(); err != nil {
		return err
	}

	attCopy := *att
	attCopy.Signature = nil
	if err := attCopy.Validate(); err != nil {
		return err
	}

	data, err := json.Marshal(&attCopy)
	if err != nil {
		return err
	}

	sigBytes, err := base64.StdEncoding.DecodeString(att.Signature.Sig)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	pubKey, err := loadPublicKey(att.Signature.PubKeyID)
	if err != nil {
		return err
	}

	if !ed25519.Verify(pubKey, data, sigBytes) {
		return fmt.Errorf("invalid attestation signature")
	}

	return nil
}

func loadPublicKey(keyID string) (ed25519.PublicKey, error) {
	if keyID == "" {
		return nil, fmt.Errorf("pubkey_id required")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	keyPath := filepath.Join(home, ".flowgate", "keys", keyID+".key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	priv := ed25519.PrivateKey(data)
	if len(priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size")
	}
	return priv.Public().(ed25519.PublicKey), nil
}
