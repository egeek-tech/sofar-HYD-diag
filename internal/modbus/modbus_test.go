package modbus

import (
	"encoding/binary"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// TestCRC16 verifies CRC-16/MODBUS against a known test vector.
// CRC16({0x01, 0x03, 0x00, 0x00, 0x00, 0x01}) = 0x0A84
func TestCRC16(t *testing.T) {
	data := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01}
	got := CRC16(data)
	want := uint16(0x0A84)
	assert.Equal(t, want, got, "CRC16(%X)", data)
}

// TestReadFull verifies that ReadFull assembles data from multiple partial reads.
func TestReadFull(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))

	want := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	// Server writes in two chunks to simulate partial reads
	go func() {
		_, _ = server.Write(want[:3]) // first 3 bytes
		_, _ = server.Write(want[3:]) // remaining 5 bytes
	}()

	buf := make([]byte, len(want))
	n, err := ReadFull(client, buf)
	require.NoError(t, err, "ReadFull error")
	assert.Equal(t, len(want), n, "ReadFull returned byte count")
	assert.Equal(t, want, buf, "ReadFull data")
}

// TestReadHoldingRegistersTCP uses net.Pipe to mock a Modbus TCP server.
// Client reads 1 register from 0x0404. Server verifies MBAP header structure
// and responds with 2 bytes of register data (0x1234).
func TestReadHoldingRegistersTCP(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))

	logger := discardLogger()

	// Capture the next txID that will be used
	nextTxID := uint16(transactionID.Load() + 1) //nolint:gosec // G115: matches production 16-bit txID generation

	go func() {
		// Read the 12-byte request
		req := make([]byte, 12)
		if _, err := ReadFull(server, req); err != nil {
			assert.NoError(t, err, "server read request")
			return
		}

		// Verify MBAP header structure
		txID := binary.BigEndian.Uint16(req[0:2])
		assert.Equal(t, nextTxID, txID, "txID")
		protocolID := binary.BigEndian.Uint16(req[2:4])
		assert.Equal(t, uint16(0x0000), protocolID, "protocol ID")
		length := binary.BigEndian.Uint16(req[4:6])
		assert.Equal(t, uint16(6), length, "MBAP length")
		assert.Equal(t, byte(0x01), req[6], "slaveID")
		assert.Equal(t, byte(0x03), req[7], "function code")

		// Build valid MBAP response: txID(2) + protocol(2) + length(2) + unitID(1) + func(1) + byteCount(1) + data(2)
		resp := make([]byte, 11)
		binary.BigEndian.PutUint16(resp[0:2], txID) // echo txID
		binary.BigEndian.PutUint16(resp[2:4], 0)    // protocol ID
		binary.BigEndian.PutUint16(resp[4:6], 5)    // length: unitID(1) + func(1) + byteCount(1) + data(2)
		resp[6] = 0x01                              // unitID
		resp[7] = 0x03                              // function code
		resp[8] = 2                                 // byte count
		resp[9] = 0x12                              // data high byte
		resp[10] = 0x34                             // data low byte

		_, _ = server.Write(resp)
	}()

	data, err := ReadHoldingRegistersTCP(client, logger, 0x01, 0x0404, 1)
	require.NoError(t, err, "ReadHoldingRegistersTCP error")
	require.Len(t, data, 2, "data length")
	assert.Equal(t, byte(0x12), data[0], "data high byte")
	assert.Equal(t, byte(0x34), data[1], "data low byte")
}

// TestReadHoldingRegistersTCP_Exception verifies that a Modbus exception response
// (function code with 0x80 bit set) returns an error containing "exception".
func TestReadHoldingRegistersTCP_Exception(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))

	logger := discardLogger()

	// Capture the next txID
	nextTxID := uint16(transactionID.Load() + 1) //nolint:gosec // G115: matches production 16-bit txID generation

	go func() {
		// Read the 12-byte request
		req := make([]byte, 12)
		if _, err := ReadFull(server, req); err != nil {
			assert.NoError(t, err, "server read request")
			return
		}

		// Build exception response: func code | 0x80, error code 0x02 (illegal data address)
		resp := make([]byte, 9)
		binary.BigEndian.PutUint16(resp[0:2], nextTxID) // txID
		binary.BigEndian.PutUint16(resp[2:4], 0)        // protocol ID
		binary.BigEndian.PutUint16(resp[4:6], 3)        // length: unitID(1) + exception func(1) + error code(1)
		resp[6] = 0x01                                  // unitID
		resp[7] = 0x83                                  // function 0x03 | 0x80
		resp[8] = 0x02                                  // error code: illegal data address

		_, _ = server.Write(resp)
	}()

	_, err := ReadHoldingRegistersTCP(client, logger, 0x01, 0x0404, 1)
	require.Error(t, err, "expected error for exception response")
	assert.Contains(t, err.Error(), "exception")
}

// TestWriteMultipleRegistersTCP uses net.Pipe to mock a Modbus TCP server.
// Client writes value 0x0100 to register 0x9020. Server verifies the 15-byte
// request structure and sends a valid write response.
func TestWriteMultipleRegistersTCP(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))

	logger := discardLogger()

	// Capture the next txID
	nextTxID := uint16(transactionID.Load() + 1) //nolint:gosec // G115: matches production 16-bit txID generation

	go func() {
		// Read the 15-byte request (MBAP 7 + PDU 8)
		req := make([]byte, 15)
		if _, err := ReadFull(server, req); err != nil {
			assert.NoError(t, err, "server read request")
			return
		}

		// Verify structure
		txID := binary.BigEndian.Uint16(req[0:2])
		assert.Equal(t, nextTxID, txID, "txID")
		assert.Equal(t, byte(0x10), req[7], "function code")
		regAddr := binary.BigEndian.Uint16(req[8:10])
		assert.Equal(t, uint16(0x9020), regAddr, "regAddr")
		qty := binary.BigEndian.Uint16(req[10:12])
		assert.Equal(t, uint16(1), qty, "quantity")
		assert.Equal(t, byte(2), req[12], "byteCount")
		value := binary.BigEndian.Uint16(req[13:15])
		assert.Equal(t, uint16(0x0100), value, "value")

		// Build valid write response: MBAP(7) + func(1) + regAddr(2) + qty(2)
		resp := make([]byte, 12)
		binary.BigEndian.PutUint16(resp[0:2], txID) // echo txID
		binary.BigEndian.PutUint16(resp[2:4], 0)    // protocol ID
		binary.BigEndian.PutUint16(resp[4:6], 6)    // length: unitID(1) + func(1) + regAddr(2) + qty(2)
		resp[6] = 0x01                              // unitID
		resp[7] = 0x10                              // function code
		binary.BigEndian.PutUint16(resp[8:10], 0x9020)
		binary.BigEndian.PutUint16(resp[10:12], 1)

		_, _ = server.Write(resp)
	}()

	err := WriteMultipleRegistersTCP(client, logger, 0x01, 0x9020, 0x0100)
	require.NoError(t, err, "WriteMultipleRegistersTCP error")
}

// TestReadHoldingRegistersRTU uses net.Pipe to mock a Modbus RTU server.
// Client reads 1 register from 0x0404. Server verifies request including CRC,
// and responds with valid RTU frame containing register data 0x1234.
func TestReadHoldingRegistersRTU(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))

	logger := discardLogger()

	go func() {
		// Read the 8-byte RTU request (6 data + 2 CRC)
		req := make([]byte, 8)
		if _, err := ReadFull(server, req); err != nil {
			assert.NoError(t, err, "server read request")
			return
		}

		// Verify CRC of the request
		reqData := req[:6]
		reqCRC := uint16(req[6]) | uint16(req[7])<<8
		calcCRC := CRC16(reqData)
		assert.Equal(t, calcCRC, reqCRC, "request CRC")

		// Verify request fields
		assert.Equal(t, byte(0x01), req[0], "slaveID")
		assert.Equal(t, byte(0x03), req[1], "function code")

		// Build valid RTU response: slaveID(1) + func(1) + byteCount(1) + data(2) + CRC(2)
		respData := []byte{
			0x01, // slaveID
			0x03, // function code
			0x02, // byte count
			0x12, // data high
			0x34, // data low
		}
		crc := CRC16(respData)
		resp := append(respData, byte(crc&0xFF), byte(crc>>8))

		_, _ = server.Write(resp)
	}()

	data, err := ReadHoldingRegistersRTU(client, logger, 0x01, 0x0404, 1)
	require.NoError(t, err, "ReadHoldingRegistersRTU error")
	require.Len(t, data, 2, "data length")
	assert.Equal(t, byte(0x12), data[0], "data high byte")
	assert.Equal(t, byte(0x34), data[1], "data low byte")
}

// TestReadHoldingRegistersRTU_CRCMismatch verifies that an RTU response with
// an incorrect CRC returns an error containing "CRC mismatch".
func TestReadHoldingRegistersRTU_CRCMismatch(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))

	logger := discardLogger()

	go func() {
		// Read the 8-byte RTU request
		req := make([]byte, 8)
		if _, err := ReadFull(server, req); err != nil {
			assert.NoError(t, err, "server read request")
			return
		}

		// Build response with intentionally bad CRC
		respData := []byte{
			0x01, // slaveID
			0x03, // function code
			0x02, // byte count
			0x12, // data high
			0x34, // data low
		}
		// Append bad CRC (0xDEAD instead of correct CRC)
		resp := append(respData, 0xAD, 0xDE)

		_, _ = server.Write(resp)
	}()

	_, err := ReadHoldingRegistersRTU(client, logger, 0x01, 0x0404, 1)
	require.Error(t, err, "expected error for CRC mismatch")
	assert.Contains(t, err.Error(), "CRC mismatch")
}
