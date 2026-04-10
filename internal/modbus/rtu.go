package modbus

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"time"
)

// ReadHoldingRegistersRTU reads holding registers via RTU (function 0x03).
// Extracted from main.go.bak lines 631-679 with exported name and slog logger.
func ReadHoldingRegistersRTU(conn net.Conn, logger *slog.Logger, slaveID byte, startAddr, quantity uint16) ([]byte, error) {
	logger.Debug("modbus rtu read", "slaveID", slaveID, "addr", fmt.Sprintf("0x%04X", startAddr), "qty", quantity)

	req := make([]byte, 6)
	req[0] = slaveID
	req[1] = 0x03
	binary.BigEndian.PutUint16(req[2:4], startAddr)
	binary.BigEndian.PutUint16(req[4:6], quantity)

	crc := CRC16(req)
	req = append(req, byte(crc&0xFF), byte(crc>>8))

	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write(req); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	header := make([]byte, 3)
	n, err := ReadFull(conn, header)
	if err != nil {
		return nil, fmt.Errorf("read header (got %d): %w", n, err)
	}

	if header[1]&0x80 != 0 {
		errBuf := make([]byte, 3)
		if _, readErr := ReadFull(conn, errBuf); readErr != nil {
			return nil, fmt.Errorf("exception (could not read error code): func=0x%02X read_err=%w", header[1], readErr)
		}
		return nil, fmt.Errorf("exception: func=0x%02X err=0x%02X", header[1], errBuf[0])
	}

	if header[0] != slaveID {
		return nil, fmt.Errorf("wrong slave: got 0x%02X want 0x%02X", header[0], slaveID)
	}

	byteCount := int(header[2])
	payload := make([]byte, byteCount+2)
	n, err = ReadFull(conn, payload)
	if err != nil {
		return nil, fmt.Errorf("read payload (got %d/%d): %w", n, byteCount+2, err)
	}

	fullResp := append(header, payload...)
	respData := fullResp[:len(fullResp)-2]
	respCRC := uint16(fullResp[len(fullResp)-2]) | uint16(fullResp[len(fullResp)-1])<<8
	calcCRC := CRC16(respData)
	if respCRC != calcCRC {
		return nil, fmt.Errorf("CRC mismatch: got=0x%04X want=0x%04X", respCRC, calcCRC)
	}

	logger.Debug("modbus rtu response", "slaveID", slaveID, "bytes", byteCount)
	return payload[:byteCount], nil
}

// WriteSingleRegisterRTU writes a single register via RTU (function 0x06).
// Extracted from main.go.bak lines 539-565 with exported name and slog logger.
func WriteSingleRegisterRTU(conn net.Conn, logger *slog.Logger, slaveID byte, regAddr, value uint16) error {
	logger.Debug("modbus rtu write", "slaveID", slaveID, "addr", fmt.Sprintf("0x%04X", regAddr), "value", value)

	req := make([]byte, 6)
	req[0] = slaveID
	req[1] = 0x06
	binary.BigEndian.PutUint16(req[2:4], regAddr)
	binary.BigEndian.PutUint16(req[4:6], value)

	crc := CRC16(req)
	req = append(req, byte(crc&0xFF), byte(crc>>8))

	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	resp := make([]byte, 8)
	if _, err := ReadFull(conn, resp); err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp[1]&0x80 != 0 {
		return fmt.Errorf("exception: func=0x%02X err=0x%02X", resp[1], resp[2])
	}

	logger.Debug("modbus rtu write response", "slaveID", slaveID)
	return nil
}
