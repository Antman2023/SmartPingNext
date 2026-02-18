package nettools

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

func randomUint16() int {
	var b [2]byte
	if _, err := rand.Read(b[:]); err == nil {
		return int(binary.BigEndian.Uint16(b[:]))
	}
	return int(uint16(time.Now().UnixNano()))
}
