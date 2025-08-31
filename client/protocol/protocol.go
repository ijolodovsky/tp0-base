package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/7574-sistemas-distribuidos/docker-compose-init/client/model"
)

// SendBet envía una sola apuesta usando el protocolo de longitud-prefijada
func SendBet(conn net.Conn, bet model.Bet) error {
	return SendBetBatch(conn, []model.Bet{bet})
}

// SendBetBatch envía un batch de apuestas usando el protocolo de longitud-prefijada
func SendBetBatch(conn net.Conn, bets []model.Bet) error {
	if len(bets) == 0 {
		return fmt.Errorf("no bets to send")
	}

	// Crear el payload con todas las apuestas separadas por "\n"
	var betStrings []string
	for _, bet := range bets {
		betStr := fmt.Sprintf("%d|%s|%s|%s|%s|%d",
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

// ReceiveAck lee los 4 bytes de confirmación del servidor para un solo bet
func ReceiveAck(conn net.Conn) (int, error) {
	buf := make([]byte, 4)
	if err := readAll(conn, buf); err != nil {
		return 0, fmt.Errorf("error reading ACK: %w", err)
	}

	ackNumber := int(binary.BigEndian.Uint32(buf))
	return ackNumber, nil
}

// ReceiveBatchAck lee la respuesta para un batch de apuestas
// Retorna true si todas las apuestas fueron procesadas correctamente, false en caso contrario
func ReceiveBatchAck(conn net.Conn) (bool, error) {
	buf := make([]byte, 1)
	if err := readAll(conn, buf); err != nil {
		return false, fmt.Errorf("error reading batch ACK: %w", err)
	}

	// 0 = error, 1 = success
	success := buf[0] == 1
	return success, nil
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