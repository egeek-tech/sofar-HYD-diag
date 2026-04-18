package modbus

import (
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestCRC16 verifies CRC-16/MODBUS against a known test vector.
// CRC16({0x01, 0x03, 0x00, 0x00, 0x00, 0x01}) = 0x0A84
func TestCRC16(t *testing.T) {
	data := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01}
	got := CRC16(data)
	want := uint16(0x0A84)
	if got != want {
		t.Errorf("CRC16(%X) = 0x%04X, want 0x%04X", data, got, want)
	}
}

// TestReadFull verifies that ReadFull assembles data from multiple partial reads.
func TestReadFull(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	client.SetDeadline(time.Now().Add(5 * time.Second))
	server.SetDeadline(time.Now().Add(5 * time.Second))

	want := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	// Server writes in two chunks to simulate partial reads
	go func() {
		server.Write(want[:3]) // first 3 bytes
		server.Write(want[3:]) // remaining 5 bytes
	}()

	buf := make([]byte, len(want))
	n, err := ReadFull(client, buf)
	if err != nil {
		t.Fatalf("ReadFull error: %v", err)
	}
	if n != len(want) {
		t.Errorf("ReadFull returned %d bytes, want %d", n, len(want))
	}
	for i, b := range buf {
		if b != want[i] {
			t.Errorf("buf[%d] = 0x%02X, want 0x%02X", i, b, want[i])
		}
	}
}

// TestReadHoldingRegistersTCP uses net.Pipe to mock a Modbus TCP server.
// Client reads 1 register from 0x0404. Server verifies MBAP header structure
// and responds with 2 bytes of register data (0x1234).
func TestReadHoldingRegistersTCP(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	client.SetDeadline(time.Now().Add(5 * time.Second))
	server.SetDeadline(time.Now().Add(5 * time.Second))

	logger := discardLogger()

	// Capture the next txID that will be used
	nextTxID := uint16(transactionID.Load() + 1)

	go func() {
		// Read the 12-byte request
		req := make([]byte, 12)
		if _, err := ReadFull(server, req); err != nil {
			t.Errorf("server read request: %v", err)
			return
		}

		// Verify MBAP header structure
		txID := binary.BigEndian.Uint16(req[0:2])
		if txID != nextTxID {
			t.Errorf("txID = %d, want %d", txID, nextTxID)
		}
		protocolID := binary.BigEndian.Uint16(req[2:4])
		if protocolID != 0x0000 {
			t.Errorf("protocol ID = 0x%04X, want 0x0000", protocolID)
		}
		length := binary.BigEndian.Uint16(req[4:6])
		if length != 6 {
			t.Errorf("MBAP length = %d, want 6", length)
		}
		if req[6] != 0x01 {
			t.Errorf("slaveID = 0x%02X, want 0x01", req[6])
		}
		if req[7] != 0x03 {
			t.Errorf("function code = 0x%02X, want 0x03", req[7])
		}

		// Build valid MBAP response: txID(2) + protocol(2) + length(2) + unitID(1) + func(1) + byteCount(1) + data(2)
		resp := make([]byte, 11)
		binary.BigEndian.PutUint16(resp[0:2], txID) // echo txID
		binary.BigEndian.PutUint16(resp[2:4], 0)    // protocol ID
		binary.BigEndian.PutUint16(resp[4:6], 5)    // length: unitID(1) + func(1) + byteCount(1) + data(2)
		resp[6] = 0x01                               // unitID
		resp[7] = 0x03                               // function code
		resp[8] = 2                                  // byte count
		resp[9] = 0x12                               // data high byte
		resp[10] = 0x34                              // data low byte

		server.Write(resp)
	}()

	data, err := ReadHoldingRegistersTCP(client, logger, 0x01, 0x0404, 1)
	if err != nil {
		t.Fatalf("ReadHoldingRegistersTCP error: %v", err)
	}
	if len(data) != 2 {
		t.Fatalf("data length = %d, want 2", len(data))
	}
	if data[0] != 0x12 || data[1] != 0x34 {
		t.Errorf("data = %X, want 1234", data)
	}
}

// TestReadHoldingRegistersTCP_Exception verifies that a Modbus exception response
// (function code with 0x80 bit set) returns an error containing "exception".
func TestReadHoldingRegistersTCP_Exception(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	client.SetDeadline(time.Now().Add(5 * time.Second))
	server.SetDeadline(time.Now().Add(5 * time.Second))

	logger := discardLogger()

	// Capture the next txID
	nextTxID := uint16(transactionID.Load() + 1)

	go func() {
		// Read the 12-byte request
		req := make([]byte, 12)
		if _, err := ReadFull(server, req); err != nil {
			t.Errorf("server read request: %v", err)
			return
		}

		// Build exception response: func code | 0x80, error code 0x02 (illegal data address)
		resp := make([]byte, 9)
		binary.BigEndian.PutUint16(resp[0:2], nextTxID) // txID
		binary.BigEndian.PutUint16(resp[2:4], 0)        // protocol ID
		binary.BigEndian.PutUint16(resp[4:6], 3)        // length: unitID(1) + exception func(1) + error code(1)
		resp[6] = 0x01                                   // unitID
		resp[7] = 0x83                                   // function 0x03 | 0x80
		resp[8] = 0x02                                   // error code: illegal data address

		server.Write(resp)
	}()

	_, err := ReadHoldingRegistersTCP(client, logger, 0x01, 0x0404, 1)
	if err == nil {
		t.Fatal("expected error for exception response, got nil")
	}
	if !strings.Contains(err.Error(), "exception") {
		t.Errorf("error = %q, want it to contain 'exception'", err.Error())
	}
}

// TestWriteMultipleRegistersTCP uses net.Pipe to mock a Modbus TCP server.
// Client writes value 0x0100 to register 0x9020. Server verifies the 15-byte
// request structure and sends a valid write response.
func TestWriteMultipleRegistersTCP(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	client.SetDeadline(time.Now().Add(5 * time.Second))
	server.SetDeadline(time.Now().Add(5 * time.Second))

	logger := discardLogger()

	// Capture the next txID
	nextTxID := uint16(transactionID.Load() + 1)

	go func() {
		// Read the 15-byte request (MBAP 7 + PDU 8)
		req := make([]byte, 15)
		if _, err := ReadFull(server, req); err != nil {
			t.Errorf("server read request: %v", err)
			return
		}

		// Verify structure
		txID := binary.BigEndian.Uint16(req[0:2])
		if txID != nextTxID {
			t.Errorf("txID = %d, want %d", txID, nextTxID)
		}
		if req[7] != 0x10 {
			t.Errorf("function code = 0x%02X, want 0x10", req[7])
		}
		regAddr := binary.BigEndian.Uint16(req[8:10])
		if regAddr != 0x9020 {
			t.Errorf("regAddr = 0x%04X, want 0x9020", regAddr)
		}
		qty := binary.BigEndian.Uint16(req[10:12])
		if qty != 1 {
			t.Errorf("quantity = %d, want 1", qty)
		}
		if req[12] != 2 {
			t.Errorf("byteCount = %d, want 2", req[12])
		}
		value := binary.BigEndian.Uint16(req[13:15])
		if value != 0x0100 {
			t.Errorf("value = 0x%04X, want 0x0100", value)
		}

		// Build valid write response: MBAP(7) + func(1) + regAddr(2) + qty(2)
		resp := make([]byte, 12)
		binary.BigEndian.PutUint16(resp[0:2], txID) // echo txID
		binary.BigEndian.PutUint16(resp[2:4], 0)    // protocol ID
		binary.BigEndian.PutUint16(resp[4:6], 6)    // length: unitID(1) + func(1) + regAddr(2) + qty(2)
		resp[6] = 0x01                               // unitID
		resp[7] = 0x10                               // function code
		binary.BigEndian.PutUint16(resp[8:10], 0x9020)
		binary.BigEndian.PutUint16(resp[10:12], 1)

		server.Write(resp)
	}()

	err := WriteMultipleRegistersTCP(client, logger, 0x01, 0x9020, 0x0100)
	if err != nil {
		t.Fatalf("WriteMultipleRegistersTCP error: %v", err)
	}
}

// TestReadHoldingRegistersRTU uses net.Pipe to mock a Modbus RTU server.
// Client reads 1 register from 0x0404. Server verifies request including CRC,
// and responds with valid RTU frame containing register data 0x1234.
func TestReadHoldingRegistersRTU(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	client.SetDeadline(time.Now().Add(5 * time.Second))
	server.SetDeadline(time.Now().Add(5 * time.Second))

	logger := discardLogger()

	go func() {
		// Read the 8-byte RTU request (6 data + 2 CRC)
		req := make([]byte, 8)
		if _, err := ReadFull(server, req); err != nil {
			t.Errorf("server read request: %v", err)
			return
		}

		// Verify CRC of the request
		reqData := req[:6]
		reqCRC := uint16(req[6]) | uint16(req[7])<<8
		calcCRC := CRC16(reqData)
		if reqCRC != calcCRC {
			t.Errorf("request CRC = 0x%04X, want 0x%04X", reqCRC, calcCRC)
		}

		// Verify request fields
		if req[0] != 0x01 {
			t.Errorf("slaveID = 0x%02X, want 0x01", req[0])
		}
		if req[1] != 0x03 {
			t.Errorf("function code = 0x%02X, want 0x03", req[1])
		}

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

		server.Write(resp)
	}()

	data, err := ReadHoldingRegistersRTU(client, logger, 0x01, 0x0404, 1)
	if err != nil {
		t.Fatalf("ReadHoldingRegistersRTU error: %v", err)
	}
	if len(data) != 2 {
		t.Fatalf("data length = %d, want 2", len(data))
	}
	if data[0] != 0x12 || data[1] != 0x34 {
		t.Errorf("data = %X, want 1234", data)
	}
}

// TestReadHoldingRegistersRTU_CRCMismatch verifies that an RTU response with
// an incorrect CRC returns an error containing "CRC mismatch".
func TestReadHoldingRegistersRTU_CRCMismatch(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	client.SetDeadline(time.Now().Add(5 * time.Second))
	server.SetDeadline(time.Now().Add(5 * time.Second))

	logger := discardLogger()

	go func() {
		// Read the 8-byte RTU request
		req := make([]byte, 8)
		if _, err := ReadFull(server, req); err != nil {
			t.Errorf("server read request: %v", err)
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

		server.Write(resp)
	}()

	_, err := ReadHoldingRegistersRTU(client, logger, 0x01, 0x0404, 1)
	if err == nil {
		t.Fatal("expected error for CRC mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "CRC mismatch") {
		t.Errorf("error = %q, want it to contain 'CRC mismatch'", err.Error())
	}
}

// TestConnect verifies Connect succeeds against a real listener and fails against a closed port.
func TestConnect(t *testing.T) {
	// Success case: connect to a real listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	conn, err := Connect(listener.Addr().String())
	if err != nil {
		t.Fatalf("Connect to listener failed: %v", err)
	}
	if conn == nil {
		t.Fatal("Connect returned nil conn without error")
	}
	conn.Close()

	// Failure case: connect to a port that refuses connections
	_, err = Connect("127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error connecting to port 1, got nil")
	}
}

// TestDiscardLogger verifies DiscardLogger returns a functional logger that does not panic.
func TestDiscardLogger(t *testing.T) {
	logger := DiscardLogger()
	if logger == nil {
		t.Fatal("DiscardLogger() returned nil")
	}
	// Should not panic
	logger.Info("test message", "key", "value")
}

// TestWriteSingleRegisterRTU uses net.Pipe to mock a Modbus RTU server.
// Client writes value 0x0100 to register 0x9020. Server verifies the request including CRC,
// and responds with a valid echo response.
func TestWriteSingleRegisterRTU(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	client.SetDeadline(time.Now().Add(5 * time.Second))
	server.SetDeadline(time.Now().Add(5 * time.Second))

	logger := discardLogger()

	go func() {
		// Read the 8-byte RTU request (6 data + 2 CRC)
		req := make([]byte, 8)
		if _, err := ReadFull(server, req); err != nil {
			t.Errorf("server read request: %v", err)
			return
		}

		// Verify request fields
		if req[0] != 0x01 {
			t.Errorf("slaveID = 0x%02X, want 0x01", req[0])
		}
		if req[1] != 0x06 {
			t.Errorf("function code = 0x%02X, want 0x06", req[1])
		}
		regAddr := binary.BigEndian.Uint16(req[2:4])
		if regAddr != 0x9020 {
			t.Errorf("regAddr = 0x%04X, want 0x9020", regAddr)
		}
		value := binary.BigEndian.Uint16(req[4:6])
		if value != 0x0100 {
			t.Errorf("value = 0x%04X, want 0x0100", value)
		}

		// Verify CRC of the request
		reqData := req[:6]
		reqCRC := uint16(req[6]) | uint16(req[7])<<8
		calcCRC := CRC16(reqData)
		if reqCRC != calcCRC {
			t.Errorf("request CRC = 0x%04X, want 0x%04X", reqCRC, calcCRC)
		}

		// Build echo response: same 6 data bytes + correct CRC (8 bytes total)
		respData := req[:6]
		crc := CRC16(respData)
		resp := make([]byte, 8)
		copy(resp[:6], respData)
		resp[6] = byte(crc & 0xFF)
		resp[7] = byte(crc >> 8)

		server.Write(resp)
	}()

	err := WriteSingleRegisterRTU(client, logger, 0x01, 0x9020, 0x0100)
	if err != nil {
		t.Fatalf("WriteSingleRegisterRTU error: %v", err)
	}
}

// TestWriteSingleRegisterRTU_Exception verifies that an RTU exception response
// (function code with 0x80 bit set) returns an error containing "exception".
func TestWriteSingleRegisterRTU_Exception(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	client.SetDeadline(time.Now().Add(5 * time.Second))
	server.SetDeadline(time.Now().Add(5 * time.Second))

	logger := discardLogger()

	go func() {
		// Read the 8-byte RTU request
		req := make([]byte, 8)
		if _, err := ReadFull(server, req); err != nil {
			t.Errorf("server read request: %v", err)
			return
		}

		// Build exception response: slaveID + (funcCode|0x80) + errorCode + padding to 8 bytes
		resp := make([]byte, 8)
		resp[0] = 0x01 // slaveID
		resp[1] = 0x86 // function 0x06 | 0x80
		resp[2] = 0x02 // error code: illegal data address
		// Remaining bytes are padding (function reads exactly 8 bytes)

		server.Write(resp)
	}()

	err := WriteSingleRegisterRTU(client, logger, 0x01, 0x9020, 0x0100)
	if err == nil {
		t.Fatal("expected error for exception response, got nil")
	}
	if !strings.Contains(err.Error(), "exception") {
		t.Errorf("error = %q, want it to contain 'exception'", err.Error())
	}
}
