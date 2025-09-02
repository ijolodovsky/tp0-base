package protocol

import (
	"fmt"
	"io"
	"net"

	"github.com/7574-sistemas-distribuidos/docker-compose-init/client/model"
)

// SendBetBatch envía un batch de apuestas usando el protocolo de longitud-prefijada
func SendBetBatch(conn net.Conn, bets []model.Bet) error {
	if len(bets) == 0 {
		return fmt.Errorf("no bets to send")
	}

	// Construir el payload manualmente, sin usar strings.Join
	payload := ""
	for i, bet := range bets {
		betStr := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
			bet.AgencyId,
			bet.Name,
			bet.LastName,
			bet.Document,
			bet.BirthDate,
			bet.Number,
		)
		payload += betStr
		if i < len(bets)-1 {
			payload += "\n"
		}
	}

	data := []byte(payload)
	length := uint16(len(data))

	// Serializamos el header (2 bytes big-endian) manualmente
	header := []byte{byte(length >> 8), byte(length & 0xFF)}

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

	// Reconstruimos el uint32 big-endian manualmente
	ackNumber := int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
	return ackNumber, nil
}

// ReceiveBatchAck lee 4 bytes: número de la última apuesta procesada exitosamente
func ReceiveBatchAck(conn net.Conn) (int, error) {
	buf := make([]byte, 4)
	if err := readAll(conn, buf); err != nil {
		return 0, fmt.Errorf("error reading batch ACK: %w", err)
	}
	
	// Reconstruimos el uint32 big-endian manualmente
	lastProcessedNumber := int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
	return lastProcessedNumber, nil
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