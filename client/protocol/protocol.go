package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/7574-sistemas-distribuidos/docker-compose-init/client/model"
)

// SendBets envía las apuestas usando el protocolo de longitud-prefijada
func SendBets(conn net.Conn, bets []model.Bet) error {
    var payload string
    for i, bet := range bets {
        line := fmt.Sprintf("%d|%s|%s|%s|%s|%d",
            bet.AgencyId,
            bet.Name,
            bet.LastName,
            bet.Document,
            bet.BirthDate,
            bet.Number,
        )
        if i > 0 {
            payload += "\n"
        }
        payload += line
    }

    data := []byte(payload)
    length := uint16(len(data))

    // Header de 2 bytes
    header := make([]byte, 2)
    binary.BigEndian.PutUint16(header, length)

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
