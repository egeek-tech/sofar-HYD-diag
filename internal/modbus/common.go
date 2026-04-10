package modbus

import (
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"time"
)

// transactionID is a global atomic counter for TCP transaction matching.
// Each TCP request increments this to get a unique ID for response matching.
var transactionID atomic.Uint32

// ReadFull reads exactly len(buf) bytes from conn, blocking until complete.
// Extracted verbatim from main.go.bak lines 681-691.
func ReadFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// CRC16 computes CRC-16/MODBUS checksum.
// Extracted verbatim from main.go.bak lines 693-705.
func CRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for range 8 {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// Connect dials a TCP connection to the given address with a 5-second timeout.
// Extracted from main.go.bak lines 67-73.
func Connect(addr string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// DiscardLogger returns a logger that discards all output (for callers that don't need logging).
func DiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
