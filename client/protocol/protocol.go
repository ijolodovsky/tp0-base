package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/7574-sistemas-distribuidos/docker-compose-init/client/model"
)

// SendBetBatch envía un batch de apuestas usando el protocolo de longitud-prefijada
func SendBetBatch(conn net.Conn, bets []model.Bet) error {
	if len(bets) == 0 {
		return fmt.Errorf("no bets to send")
	}

	// Crear el payload con todas las apuestas separadas por "\n"
	var betStrings []string
	for _, bet := range bets {
		betStr := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
			bet.AgencyId,
			bet.Name,
			bet.LastName,
			bet.Document,
			bet.BirthDate,
			bet.Number,
		)
		betStrings = append(betStrings, betStr)
	}

	payload := strings.Join(betStrings, "\n")
	data := []byte(payload)
	length := uint16(len(data))

	// Header de 2 bytes big-endian
	header := make([]byte, 2)
	binary.BigEndian.PutUint16(header, length)

	// Primero el header, y despues el payload
	if err := writeAll(conn, header); err != nil {
		return fmt.Errorf("error sending header: %w", err)
	}
	if err := writeAll(conn, data); err != nil {
		return fmt.Errorf("error sending payload: %w", err)
	}

	return nil
}

// ReceiveAck lee los 4 bytes de confirmación del servidor para un solo bet
func ReceiveAck(conn net.Conn) (int, error) {
	buf := make([]byte, 4)
	if err := readAll(conn, buf); err != nil {
		return 0, fmt.Errorf("error reading ACK: %w", err)
	}

	ackNumber := int(binary.BigEndian.Uint32(buf))
	return ackNumber, nil
}

// ReceiveBatchAck lee 1 byte: 1=éxito del batch, 0=fallo
func ReceiveBatchAck(conn net.Conn) (bool, error) {
	buf := make([]byte, 1)
	if err := readAll(conn, buf); err != nil {
		return false, fmt.Errorf("error reading batch ACK: %w", err)
	}
	return buf[0] == 1, nil
}

func writeAll(conn net.Conn, data []byte) error {
	total := 0
	for total < len(data) {
		n, err := conn.Write(data[total:])
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}

func readAll(conn net.Conn, buf []byte) error {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		if err != nil {
			if err == io.EOF && total > 0 {
				return fmt.Errorf("unexpected EOF, read %d bytes of %d", total, len(buf))
			}
			return err
		}
		total += n
	}
	return nil
}
