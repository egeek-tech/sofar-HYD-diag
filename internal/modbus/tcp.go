package modbus

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"time"
)

// ReadHoldingRegistersTCP reads holding registers via TCP (function 0x03).
// Extracted from main.go.bak lines 567-629 with exported name and slog logger.
func ReadHoldingRegistersTCP(conn net.Conn, logger *slog.Logger, slaveID byte, startAddr, quantity uint16) ([]byte, error) {
	logger.Debug("modbus tcp read", "slaveID", slaveID, "addr", fmt.Sprintf("0x%04X", startAddr), "qty", quantity)

	txID := uint16(transactionID.Add(1))

	req := make([]byte, 12)
	binary.BigEndian.PutUint16(req[0:2], txID)
	binary.BigEndian.PutUint16(req[2:4], 0)
	binary.BigEndian.PutUint16(req[4:6], 6)
	req[6] = slaveID
	req[7] = 0x03
	binary.BigEndian.PutUint16(req[8:10], startAddr)
	binary.BigEndian.PutUint16(req[10:12], quantity)

	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write(req); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// Read response, matching transaction ID to skip stale responses
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		mbap := make([]byte, 7)
		if _, err := ReadFull(conn, mbap); err != nil {
			return nil, fmt.Errorf("timeout: %w", err)
		}

		respTxID := binary.BigEndian.Uint16(mbap[0:2])
		respLen := int(binary.BigEndian.Uint16(mbap[4:6]))
		if respLen < 2 || respLen > 260 {
			return nil, fmt.Errorf("invalid MBAP length: %d", respLen)
		}

		pdu := make([]byte, respLen-1)
		if _, err := ReadFull(conn, pdu); err != nil {
			return nil, fmt.Errorf("read PDU: %w", err)
		}

		if respTxID != txID {
			continue // skip stale response from previous request
		}

		if pdu[0]&0x80 != 0 {
			errCode := byte(0)
			if len(pdu) > 1 {
				errCode = pdu[1]
			}
			return nil, fmt.Errorf("exception: func=0x%02X err=0x%02X", pdu[0], errCode)
		}

		if len(pdu) < 2 {
			return nil, fmt.Errorf("PDU too short")
		}

		byteCount := int(pdu[1])
		if len(pdu) < 2+byteCount {
			return nil, fmt.Errorf("PDU data short")
		}

		logger.Debug("modbus tcp response", "txID", txID, "bytes", byteCount)
		return pdu[2 : 2+byteCount], nil
	}

	return nil, fmt.Errorf("timeout waiting for matching response (txID=%d)", txID)
}

// WriteMultipleRegistersTCP writes a single register using function 0x10 (Write Multiple Registers).
// Function 0x06 (Write Single Register) does NOT work for 0x9020 on this inverter - times out.
// Function 0x10 works and is confirmed tested.
// Extracted from main.go.bak lines 481-537 with exported name and slog logger.
func WriteMultipleRegistersTCP(conn net.Conn, logger *slog.Logger, slaveID byte, regAddr, value uint16) error {
	logger.Debug("modbus tcp write", "slaveID", slaveID, "addr", fmt.Sprintf("0x%04X", regAddr), "value", value)

	txID := uint16(transactionID.Add(1))

	// MBAP header (7 bytes) + PDU: funcCode(1) + regAddr(2) + quantity(2) + byteCount(1) + data(2) = 8
	req := make([]byte, 15)
	binary.BigEndian.PutUint16(req[0:2], txID)   // transaction ID
	binary.BigEndian.PutUint16(req[2:4], 0)       // protocol ID
	binary.BigEndian.PutUint16(req[4:6], 9)       // length: unitID(1) + PDU(8)
	req[6] = slaveID
	req[7] = 0x10 // Write Multiple Registers
	binary.BigEndian.PutUint16(req[8:10], regAddr)
	binary.BigEndian.PutUint16(req[10:12], 1) // quantity = 1 register
	req[12] = 2                                // byte count = 2
	binary.BigEndian.PutUint16(req[13:15], value)

	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	// Read response, matching transaction ID (skip stale responses)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		mbap := make([]byte, 7)
		if _, err := ReadFull(conn, mbap); err != nil {
			return fmt.Errorf("read MBAP: %w", err)
		}

		respTxID := binary.BigEndian.Uint16(mbap[0:2])
		respLen := int(binary.BigEndian.Uint16(mbap[4:6]))
		if respLen < 1 || respLen > 260 {
			return fmt.Errorf("invalid MBAP length: %d", respLen)
		}

		pdu := make([]byte, respLen-1)
		if _, err := ReadFull(conn, pdu); err != nil {
			return fmt.Errorf("read PDU: %w", err)
		}

		if respTxID != txID {
			continue // skip stale response
		}

		if len(pdu) > 0 && pdu[0]&0x80 != 0 {
			errCode := byte(0)
			if len(pdu) > 1 {
				errCode = pdu[1]
			}
			return fmt.Errorf("exception: func=0x%02X err=0x%02X", pdu[0], errCode)
		}

		logger.Debug("modbus tcp write response", "txID", txID)
		return nil // success
	}

	return fmt.Errorf("timeout waiting for write response (txID=%d)", txID)
}
