package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func CanonicalHash(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
