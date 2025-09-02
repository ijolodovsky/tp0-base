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

	// Construir el payload manualmente, sin strings.Join
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

	// Header de 2 bytes big-endian manual
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

// SendFinishConfirmation envía mensaje cuando termina el cliente de enviar todas sus apuestas (cuando no hay mas batches)
func SendFinishConfirmation(conn net.Conn, agencyId string) error {
	payload := "FIN_APUESTAS|" + agencyId
	data := []byte(payload)
	length := uint16(len(data))

	header := []byte{byte(length >> 8), byte(length & 0xFF)}

	if err := writeAll(conn, header); err != nil {
		return fmt.Errorf("error sending finish confirmation header: %w", err)
	}
	if err := writeAll(conn, data); err != nil {
		return fmt.Errorf("error sending finish confirmation payload: %w", err)
	}

	return nil
}

func SendWinnersQuery(conn net.Conn, agencyId string) error {
	payload := "CONSULTA_GANADORES|" + agencyId
	data := []byte(payload)
	length := uint16(len(data))

	header := []byte{byte(length >> 8), byte(length & 0xFF)}

	if err := writeAll(conn, header); err != nil {
		return fmt.Errorf("error sending winners query header: %w", err)
	}
	if err := writeAll(conn, data); err != nil {
		return fmt.Errorf("error sending winners query payload: %w", err)
	}

	return nil
}

// ReceiveAck lee los 4 bytes de confirmación del servidor para un solo bet
func ReceiveAck(conn net.Conn) (int, error) {
	buf := make([]byte, 4)
	if err := readAll(conn, buf); err != nil {
		return 0, fmt.Errorf("error reading ACK: %w", err)
	}

	ackNumber := int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
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

// ReceiveFinishAck lee 1 byte: 1=éxito de la confirmación de finalización, 0=fallo
func ReceiveFinishAck(conn net.Conn) (bool, error) {
	buf := make([]byte, 1)
	if err := readAll(conn, buf); err != nil {
		return false, fmt.Errorf("error reading finish ACK: %w", err)
	}
	return buf[0] == 1, nil
}

// ReceiveWinnersList lee la lista de DNI ganadores del servidor
func ReceiveWinnersList(conn net.Conn) ([]string, error) {
	// Leer header de 2 bytes
	header := make([]byte, 2)
	if err := readAll(conn, header); err != nil {
		return nil, fmt.Errorf("error reading winners list header: %w", err)
	}

	// Obtener longitud del payload manualmente
	length := int(header[0])<<8 | int(header[1])

	// Leer payload
	data := make([]byte, length)
	if err := readAll(conn, data); err != nil {
		return nil, fmt.Errorf("error reading winners list data: %w", err)
	}

	response := string(data)

	// Manejar respuestas especiales
	if response == "ERROR_NO_SORTEO" {
		return nil, fmt.Errorf("no draw has been conducted")
	}

	if response == "" {
		return []string{}, nil
	}

	// Parsear lista de DNIs separados por "|"
	winners := []string{}
	start := 0
	for i := 0; i < len(response); i++ {
		if response[i] == '|' {
			winners = append(winners, response[start:i])
			start = i + 1
		}
	}
	if start <= len(response)-1 {
		winners = append(winners, response[start:])
	}
	return winners, nil
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