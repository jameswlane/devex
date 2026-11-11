package remote_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/backup/remote"
)

func remoteManifest(id string, payload []byte) []byte {
	sum := sha256.Sum256(payload)
	data, _ := json.Marshal(remote.Manifest{ID: id, Encrypted: true, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(payload)), PushedAt: time.Now()})
	return data
}
