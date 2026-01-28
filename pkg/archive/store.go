package archive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zen-systems/flowgate/pkg/schema"
)

// Store manages the content-addressed archive.
type Store struct {
	BasePath string
}

// NewStore creates a new archive store.
func NewStore(basePath string) (*Store, error) {
	if basePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		basePath = filepath.Join(home, "flowgate", "archive")
	}

	dirs := []string{
		filepath.Join(basePath, "objects"),
		filepath.Join(basePath, "attestations"),
		filepath.Join(basePath, "indexes"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, err
		}
	}

	return &Store{BasePath: basePath}, nil
}

// StoreObject stores a JSON object by its SHA256 content hash in a sharded directory structure.
func (s *Store) StoreObject(obj any, kind string) (schema.EvidenceRef, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return schema.EvidenceRef{}, err
	}

	hashBytes := sha256.Sum256(data)
	hash := hex.EncodeToString(hashBytes[:])

	// Shard by first 2 chars
	shard := hash[:2]
	dir := filepath.Join(s.BasePath, "objects", shard)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return schema.EvidenceRef{}, err
	}

	path := filepath.Join(dir, hash+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return schema.EvidenceRef{}, err
	}

	return schema.EvidenceRef{
		Kind:   kind,
		SHA256: hash,
	}, nil
}

// StoreBlob stores raw bytes (e.g. output code) and returns a ref.
func (s *Store) StoreBlob(data []byte) (schema.EvidenceRef, error) {
	hashBytes := sha256.Sum256(data)
	hash := hex.EncodeToString(hashBytes[:])

	shard := hash[:2]
	dir := filepath.Join(s.BasePath, "objects", shard)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return schema.EvidenceRef{}, err
	}

	// Don't append .json for raw blobs, maybe just hash or hash.bin
	path := filepath.Join(dir, hash)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return schema.EvidenceRef{}, err
	}

	return schema.EvidenceRef{
		Kind:   "output",
		SHA256: hash,
	}, nil
}

// StoreAttestation stores a signed attestation in the attestations directory.
func (s *Store) StoreAttestation(att *schema.AttestationV1) error {
	data, err := json.MarshalIndent(att, "", "  ") // Indent for human readability
	if err != nil {
		return err
	}

	// Naming: timestamp__attestationID.json
	timestamp := time.Unix(att.Provenance.Timestamp, 0).Format("20060102150405")
	filename := fmt.Sprintf("%s__%s.json", timestamp, att.AttestationID)

	path := filepath.Join(s.BasePath, "attestations", filename)
	return os.WriteFile(path, data, 0644)
}
