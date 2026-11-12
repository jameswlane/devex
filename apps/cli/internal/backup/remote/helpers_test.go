package remote_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/backup/remote"
)

// publishRaw stores payload as remote backup id under prefix with a matching
// manifest, as a push would, so specs can serve hand-built archives.
func publishRaw(objects map[string][]byte, prefix, id string, payload []byte) {
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	data, _ := json.Marshal(remote.Manifest{ID: id, Encrypted: true, SHA256: digest, Size: int64(len(payload)), PushedAt: time.Now()})
	objects[prefix+id+".json"] = data
	objects[prefix+id+"."+digest+".devexbak"] = payload
}
