package data

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/J-Y-Zhang/light-cloud-disk/app/file/internal/biz"
	"github.com/klauspost/reedsolomon"
)

// rsEncoder wraps the package-level Reed-Solomon helpers
// and implements biz.ErasureEncoder.
type rsEncoder struct{}

// NewErasureEncoder returns a biz.ErasureEncoder backed by klauspost/reedsolomon.
func NewErasureEncoder() biz.ErasureEncoder { return &rsEncoder{} }

func (r *rsEncoder) Encode(filePath, storeDir, fileMD5 string, dataShards, parityShards int) ([]string, error) {
	return EncodeErasure(filePath, storeDir, fileMD5, dataShards, parityShards)
}

func (r *rsEncoder) Reconstruct(shardPaths []string, dataShards, parityShards int) ([]byte, error) {
	return ReconstructErasure(shardPaths, dataShards, parityShards)
}

func (r *rsEncoder) ShardChecksum(path string) (string, error) {
	return shardChecksum(path)
}

// EncodeErasure splits a local file into data+parity shards using Reed-Solomon.
// Returns the shard file paths (data first, then parity).
//
// Shard naming convention: {storeDir}/{md5}.shard.{index}
// where index 0..dataShards-1 are data and dataShards..total-1 are parity.
func EncodeErasure(filePath, storeDir, fileMD5 string, dataShards, parityShards int) ([]string, error) {
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, fmt.Errorf("reedsolomon.New: %w", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read source file: %w", err)
	}

	shards, err := enc.Split(data)
	if err != nil {
		return nil, fmt.Errorf("split into shards: %w", err)
	}

	if err := enc.Encode(shards); err != nil {
		return nil, fmt.Errorf("encode parity: %w", err)
	}

	total := dataShards + parityShards
	paths := make([]string, total)
	for i := 0; i < total; i++ {
		p := filepath.Join(storeDir, fmt.Sprintf("%s.shard.%d", fileMD5, i))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, shards[i], 0o644); err != nil {
			return nil, fmt.Errorf("write shard %d: %w", i, err)
		}
		paths[i] = p
	}

	return paths, nil
}

// ReconstructErasure reads available shards, reconstructs any missing ones,
// and returns the combined original data.
// shardPaths is a full list of shard file paths (data + parity).
// Missing or corrupted shards are detected automatically.
func ReconstructErasure(shardPaths []string, dataShards, parityShards int) ([]byte, error) {
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, fmt.Errorf("reedsolomon.New: %w", err)
	}

	total := dataShards + parityShards
	shards := make([][]byte, total)
	missing := 0

	for i := 0; i < total; i++ {
		if i >= len(shardPaths) || shardPaths[i] == "" {
			missing++
			continue
		}
		data, err := os.ReadFile(shardPaths[i])
		if err != nil {
			// Treat unreadable shard as missing
			missing++
			continue
		}
		shards[i] = data
	}

	if missing > parityShards {
		return nil, fmt.Errorf("too many missing shards: %d missing, max recoverable %d", missing, parityShards)
	}

	if missing > 0 {
		if err := enc.Reconstruct(shards); err != nil {
			return nil, fmt.Errorf("reconstruct: %w", err)
		}
	}

	ok, err := enc.Verify(shards)
	if err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("verification failed after reconstruction")
	}

	// Join data shards into the original file
	var buf []byte
	for i := 0; i < dataShards; i++ {
		buf = append(buf, shards[i]...)
	}

	return buf, nil
}

// shardChecksum returns the MD5 hex digest of a shard file.
func shardChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
