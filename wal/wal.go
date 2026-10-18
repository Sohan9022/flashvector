package wal

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"io"
	"os"
	"sync"
)

const (
	opSet    byte = 1
	opDelete byte = 2
)

type Replayer interface {
	ApplySet(key string, value []byte, metadata map[string]string)
	ApplyDelete(key string)
}

type WAL struct {
	mu   sync.Mutex
	file *os.File
}

func Open(path string) (*WAL, error) {
	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_RDWR,
		0644,
	)
	if err != nil {
		return nil, err
	}

	return &WAL{
		file: file,
	}, nil
}

// LogSet writes a Set operation to the WAL with binary length prefixes and CRC32 checksum
func (w *WAL) LogSet(key string, value []byte, metadata map[string]string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var metaBytes []byte
	var err error
	if metadata != nil {
		metaBytes, err = json.Marshal(metadata)
		if err != nil {
			return err
		}
	}

	keylen := uint32(len(key))
	valuelen := uint32(len(value))
	metalen := uint32(len(metaBytes))

	// Buffer record header and payload for atomic write
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, opSet)
	_ = binary.Write(buf, binary.LittleEndian, keylen)
	_ = binary.Write(buf, binary.LittleEndian, valuelen)
	_ = binary.Write(buf, binary.LittleEndian, metalen)
	buf.WriteString(key)
	buf.Write(value)
	if metalen > 0 {
		buf.Write(metaBytes)
	}

	// Calculate IEEE CRC32 checksum over the entire entry
	checksum := crc32.ChecksumIEEE(buf.Bytes())
	_ = binary.Write(buf, binary.LittleEndian, checksum)

	if _, err := w.file.Write(buf.Bytes()); err != nil {
		return err
	}

	return w.file.Sync()
}

// LogDelete writes a Delete tombstone operation to the WAL with CRC32 checksum
func (w *WAL) LogDelete(key string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	keylen := uint32(len(key))
	var vallen uint32 = 0

	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, opDelete)
	_ = binary.Write(buf, binary.LittleEndian, keylen)
	_ = binary.Write(buf, binary.LittleEndian, vallen)
	buf.WriteString(key)

	checksum := crc32.ChecksumIEEE(buf.Bytes())
	_ = binary.Write(buf, binary.LittleEndian, checksum)

	if _, err := w.file.Write(buf.Bytes()); err != nil {
		return err
	}

	return w.file.Sync()
}

// Replay reads the WAL from start, verifies CRC32 checksums, and truncates any corrupted tails
func (w *WAL) Replay(r Replayer) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	var lastGoodOffset int64 = 0

	for {
		var op byte
		var keylen, vallen, metalen, storedChecksum uint32
		var key, value, metaBuf []byte
		var err error

		hasher := crc32.NewIEEE()

		// 1. Read Op
		if err = binary.Read(w.file, binary.LittleEndian, &op); err != nil {
			if err == io.EOF {
				break
			}
			goto TRUNCATE
		}
		_ = binary.Write(hasher, binary.LittleEndian, op)

		// 2. Read Key Length
		if err = binary.Read(w.file, binary.LittleEndian, &keylen); err != nil {
			goto TRUNCATE
		}
		_ = binary.Write(hasher, binary.LittleEndian, keylen)

		// 3. Read Value Length
		if err = binary.Read(w.file, binary.LittleEndian, &vallen); err != nil {
			goto TRUNCATE
		}
		_ = binary.Write(hasher, binary.LittleEndian, vallen)

		// 4. Read Metadata Length (only for opSet)
		if op == opSet {
			if err = binary.Read(w.file, binary.LittleEndian, &metalen); err != nil {
				goto TRUNCATE
			}
			_ = binary.Write(hasher, binary.LittleEndian, metalen)
		} else if op != opDelete {
			goto TRUNCATE
		}

		// 5. Read Key Bytes
		key = make([]byte, keylen)
		if _, err = io.ReadFull(w.file, key); err != nil {
			goto TRUNCATE
		}
		hasher.Write(key)

		// 6. Read Value Bytes
		if vallen > 0 {
			value = make([]byte, vallen)
			if _, err = io.ReadFull(w.file, value); err != nil {
				goto TRUNCATE
			}
			hasher.Write(value)
		}

		// 7. Read Metadata Bytes
		var metaMap map[string]string
		if op == opSet && metalen > 0 {
			metaBuf = make([]byte, metalen)
			if _, err = io.ReadFull(w.file, metaBuf); err != nil {
				goto TRUNCATE
			}
			hasher.Write(metaBuf)
			if err = json.Unmarshal(metaBuf, &metaMap); err != nil {
				goto TRUNCATE
			}
		}

		// 8. Read and Validate CRC32 Checksum
		if err = binary.Read(w.file, binary.LittleEndian, &storedChecksum); err != nil {
			goto TRUNCATE
		}
		if storedChecksum != hasher.Sum32() {
			// Checksum mismatch! Corrupted record detected.
			goto TRUNCATE
		}

		// 9. Apply to Store
		switch op {
		case opSet:
			r.ApplySet(string(key), value, metaMap)
		case opDelete:
			r.ApplyDelete(string(key))
		default:
			goto TRUNCATE
		}

		// 10. Checkpoint valid offset
		currOffset, err := w.file.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		lastGoodOffset = currOffset
	}

	return nil

TRUNCATE:
	if err := w.file.Truncate(lastGoodOffset); err != nil {
		return err
	}
	_, err := w.file.Seek(0, io.SeekEnd)
	return err
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		_ = w.file.Sync()
		return w.file.Close()
	}
	return nil
}

func (w *WAL) Reset() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.file.Truncate(0); err != nil {
		return err
	}
	_, err := w.file.Seek(0, 0)
	return err
}