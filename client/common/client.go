package common

import (
	"net"
	"os"

	"github.com/7574-sistemas-distribuidos/docker-compose-init/client/model"
	"github.com/7574-sistemas-distribuidos/docker-compose-init/client/protocol"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

type ClientConfig struct {
	ID             string
	ServerAddress  string
	BatchMaxAmount int
}

type Client struct {
	config ClientConfig
	bets   []model.Bet
	conn   net.Conn
}

func NewClient(config ClientConfig, bets []model.Bet) *Client {
	return &Client{
		config: config,
		bets:   bets,
	}
}

// createClientSocket inicializa la conexión
func (c *Client) createClientSocket() error {
	log.Debugf("intentando conectar a %s", c.config.ServerAddress)
	conn, err := net.Dial("tcp", c.config.ServerAddress)
	if err != nil {
		log.Errorf("action: connect | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return err
	}
	log.Debugf("conexión exitosa a %s", c.config.ServerAddress)
	c.conn = conn
	return nil
}

// StartClient inicia el cliente y maneja la señal de apagado
func (c *Client) StartClientLoop(sigChan chan os.Signal) {
	log.Infof("[DEBUG] BatchMaxAmount usado por el cliente: %d", c.config.BatchMaxAmount)
	select {
	case <-sigChan:
		log.Infof("action: shutdown | result: success")
		return
	default:
		if err := c.createClientSocket(); err != nil {
			return
		}
		//se pospone la ejecucion hasta el final de la función.
		defer c.conn.Close()

		totalBetsSent := 0
		success := true

		for i := 0; i < len(c.bets); i += c.config.BatchMaxAmount {
			end := i + c.config.BatchMaxAmount
			if end > len(c.bets) {
				end = len(c.bets)
			}
			chunk := c.bets[i:end]

			if err := protocol.SendBets(c.conn, chunk); err != nil {
				log.Errorf("action: send_bets | result: fail | client_id: %v | error: %v", c.config.ID, err)
				success = false
				break
			}

			// se usa ReceiveAck de protocol
			ack, err := protocol.ReceiveAck(c.conn)
			if err != nil {
				// Si es EOF y es el último batch, considerarlo éxito
				if err.Error() == "EOF" && end == len(c.bets) {
					totalBetsSent += len(chunk)
					break
				}
				log.Errorf("action: receive_ack | result: fail | client_id: %v | error: %v", c.config.ID, err)
				success = false
				break
			}

			// Verificar que el ACK corresponda a la última apuesta del chunk
			lastBet := chunk[len(chunk)-1]
			if ack == lastBet.Number {
				totalBetsSent += len(chunk)
			} else {
				log.Errorf("action: apuestas_enviadas | result: fail | client_id: %v | expected: %d | received: %d", c.config.ID, lastBet.Number, ack)
				success = false
				break
			}
		}

		// Log del resultado final
		if success {
			log.Infof("action: apuestas_enviadas | result: success | cantidad: %d | client_id: %v", totalBetsSent, c.config.ID)
		} else {
			log.Errorf("action: apuestas_enviadas | result: fail | cantidad: %d | client_id: %v", totalBetsSent, c.config.ID)
		}
	}
}

func (c *Client) Stop() {
	if c.conn != nil {
		c.conn.Close()
		log.Infof("action: close_connection | result: success | client_id: %v", c.config.ID)
	}

	log.Infof("action: exit | result: success | client_id: %v", c.config.ID)
}
