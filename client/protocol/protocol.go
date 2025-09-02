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

	// Armo el payload juntando todas las apuestas
	// Formato: cada apuesta es "agencia|nombre|apellido|dni|fecha|numero"
	// Las apuestas se separan con \n
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

	// Convierto a bytes y calculo la longitud
	data := []byte(payload)
	length := uint16(len(data))

	// Protocolo: 2 bytes de header (longitud) + payload
	// Big-endian = byte más significativo primero
	header := []byte{byte(length >> 8), byte(length & 0xFF)}

	// Envío primero el header, después el contenido
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

// Pido al servidor la lista de ganadores de mi agencia
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

// Recibo confirmación del servidor después de enviar un batch
// Me dice hasta qué número de apuesta procesó bien
func ReceiveBatchAck(conn net.Conn) (int, error) {
	buf := make([]byte, 4)
	if err := readAll(conn, buf); err != nil {
		return 0, fmt.Errorf("error reading batch ACK: %w", err)
	}
	
	// Decodifico 4 bytes big-endian a int
	lastProcessedNumber := int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
	return lastProcessedNumber, nil
}

// Recibo confirmación de que el servidor recibió mi notificación de fin
func ReceiveFinishAck(conn net.Conn) (bool, error) {
	buf := make([]byte, 1)
	if err := readAll(conn, buf); err != nil {
		return false, fmt.Errorf("error reading finish ACK: %w", err)
	}
	// 1 byte: 1 = exito, 0 = error
	return buf[0] == 1, nil
}

// Recibo la lista de ganadores de mi agencia
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
		// No hay ganadores en mi agencia
		return []string{}, nil
	}

	// Parseo la lista de DNIs separados por |
	// Ejemplo: "12345678|87654321|11111111"
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
	// Sigo enviando hasta mandar todos los bytes
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
	// Sigo leyendo hasta llenar todo el buffer
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