package protocol

import (
	"encoding/binary"
	"fmt"
	"net"
)

// SendBet envía la apuesta usando el protocolo de longitud-prefijada
func SendBet(conn net.Conn, bet Bet) error {
	// Armamos el payload como string separado por "|"
	payload := fmt.Sprintf("%d|%s|%s|%s|%s|%d",
		bet.Agency,
		bet.Name,
		bet.Surname,
		bet.DocNumber,
		bet.BirthDate,
		bet.Number,
	)

	data := []byte(payload)
	length := uint16(len(data)) // max 65535 bytes

	// Preparamos el header de 2 bytes big-endian
	header := make([]byte, 2)
	binary.BigEndian.PutUint16(header, length)

	// Enviamos primero el header, luego el payload
	if _, err := conn.Write(header); err != nil {
		return err
	}
	if _, err := conn.Write(data); err != nil {
		return err
	}

	return nil
}

// ReceiveAck lee los 4 bytes de confirmación del servidor
func ReceiveAck(conn net.Conn) (int, error) {
	buf := make([]byte, 4)
	n, err := conn.Read(buf)
	if err != nil {
		return 0, err
	}
	if n != 4 {
		return 0, fmt.Errorf("expected 4 bytes ACK, got %d", n)
	}

	ackNumber := int(binary.BigEndian.Uint32(buf))
	return ackNumber, nil
}
