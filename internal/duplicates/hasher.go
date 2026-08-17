package duplicates

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// quickHashBytes is how much of a file's start is hashed during the
// cheap filtering pass. Large enough to reject most non-duplicates
// without a full read; small enough that hashing every candidate in a
// size bucket stays fast even for many-GB files.
const quickHashBytes = 64 * 1024

func quickHash(path string, size int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	limit := size
	if limit > quickHashBytes {
		limit = quickHashBytes
	}

	h := sha256.New()
	if _, err := io.CopyN(h, f, limit); err != nil && err != io.EOF {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fullHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
