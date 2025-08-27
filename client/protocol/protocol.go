package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// SendBet envía la apuesta usando el protocolo de longitud-prefijada
func SendBet(conn net.Conn, bet Bet) error {
	// Armamos el payload como string separado por "|" en el orden compatible con Python
	payload := fmt.Sprintf("%d|%s|%s|%s|%s|%d",
		bet.Agency,     // agency
		bet.FirstName,  // first_name
		bet.LastName,   // last_name
		bet.Document,   // document (string)
		bet.BirthDate,  // birthdate (YYYY-MM-DD)
		bet.Number,     // number
	)

	data := []byte(payload)
	length := uint16(len(data)) // max 65535 bytes

	// Preparamos el header de 2 bytes big-endian
	header := make([]byte, 2)
	binary.BigEndian.PutUint16(header, length)

	// Enviamos primero el header, luego el payload
	if err := writeAll(conn, header); err != nil {
		return fmt.Errorf("error sending header: %w", err)
	}
	if err := writeAll(conn, data); err != nil {
		return fmt.Errorf("error sending payload: %w", err)
	}

	return nil
}

// ReceiveAck lee los 4 bytes de confirmación del servidor
func ReceiveAck(conn net.Conn) (int, error) {
	buf := make([]byte, 4)
	if err := readAll(conn, buf); err != nil {
		return 0, fmt.Errorf("error reading ACK: %w", err)
	}

	ackNumber := int(binary.BigEndian.Uint32(buf))
	return ackNumber, nil
}

func writeAll(conn net.Conn, data []byte) error {
	totalWritten := 0
	for totalWritten < len(data) {
		n, err := conn.Write(data[totalWritten:])
		if err != nil {
			return err
		}
		totalWritten += n
	}
	return nil
}

func readAll(conn net.Conn, buf []byte) error {
	totalRead := 0
	for totalRead < len(buf) {
		n, err := conn.Read(buf[totalRead:])
		if err != nil {
			if err == io.EOF && totalRead > 0 {
				return fmt.Errorf("unexpected EOF, read %d bytes of %d", totalRead, len(buf))
			}
			return err
		}
		totalRead += n
	}
	return nil
}