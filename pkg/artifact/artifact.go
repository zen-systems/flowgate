package artifact

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"time"
)

// Artifact represents an immutable, versioned output from an LLM.
type Artifact struct {
	ID        string            `json:"id"`
	Version   int               `json:"version"`
	Content   string            `json:"content"`
	Adapter   string            `json:"adapter"`
	Model     string            `json:"model"`
	Prompt    string            `json:"prompt"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	Hash      string            `json:"hash"`
}

// New creates a new Artifact with computed hash.
func New(content, adapter, model, prompt string) *Artifact {
	a := &Artifact{
		ID:        generateID(),
		Version:   1,
		Content:   content,
		Adapter:   adapter,
		Model:     model,
		Prompt:    prompt,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now().UTC(),
	}
	a.Hash = a.computeHash()
	return a
}

// NewVersion creates a new version of an artifact with updated content.
func (a *Artifact) NewVersion(content string) *Artifact {
	newArtifact := &Artifact{
		ID:        a.ID,
		Version:   a.Version + 1,
		Content:   content,
		Adapter:   a.Adapter,
		Model:     a.Model,
		Prompt:    a.Prompt,
		Metadata:  copyMetadata(a.Metadata),
		CreatedAt: time.Now().UTC(),
	}
	newArtifact.Hash = newArtifact.computeHash()
	return newArtifact
}

// WithMetadata returns a new artifact with additional metadata.
func (a *Artifact) WithMetadata(key, value string) *Artifact {
	newArtifact := &Artifact{
		ID:        a.ID,
		Version:   a.Version,
		Content:   a.Content,
		Adapter:   a.Adapter,
		Model:     a.Model,
		Prompt:    a.Prompt,
		Metadata:  copyMetadata(a.Metadata),
		CreatedAt: a.CreatedAt,
		Hash:      a.Hash,
	}
	newArtifact.Metadata[key] = value
	return newArtifact
}

func (a *Artifact) computeHash() string {
	h := sha256.New()
	h.Write([]byte(a.Content))
	h.Write([]byte(a.Adapter))
	h.Write([]byte(a.Model))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func generateID() string {
	h := sha256.New()
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(time.Now().UnixNano()))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func copyMetadata(m map[string]string) map[string]string {
	newM := make(map[string]string, len(m))
	for k, v := range m {
		newM[k] = v
	}
	return newM
}
