package common

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/op/go-logging"
	"github.com/7574-sistemas-distribuidos/docker-compose-init/client/protocol"
)

var log = logging.MustGetLogger("log")

type ClientConfig struct {
	ID            string
	ServerAddress string
}

type Client struct {
	config ClientConfig
	bet    Bet
	conn   net.Conn
}

func NewClient(config ClientConfig, bet Bet) *Client {
	return &Client{
		config: config,
		bet:    bet,
	}
}

// createClientSocket inicializa la conexión
func (c *Client) createClientSocket() error {
	conn, err := net.Dial("tcp", c.config.ServerAddress)
	if err != nil {
		log.Errorf("action: connect | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return err
	}
	c.conn = conn
	return nil
}

// StartClient inicia el cliente y maneja la señal de apagado
func (c *Client) StartClient(sigChan chan os.Signal) {
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

		// se usa SendBet de protocol
		if err := protocol.SendBet(c.conn, c.bet); err != nil {
			log.Errorf("action: send_bet | result: fail | client_id: %v | error: %v", c.config.ID, err)
			return
		}

		// se usa ReceiveAck de protocol
		ack, err := protocol.ReceiveAck(c.conn)
		if err != nil {
			log.Errorf("action: receive_ack | result: fail | client_id: %v | error: %v", c.config.ID, err)
			return
		}

		if ack == c.bet.Number {
			log.Infof("action: apuesta_enviada | result: success | dni: %s | numero: %d", c.bet.DocNumber, c.bet.Number)
		} else {
			log.Errorf("action: apuesta_enviada | result: fail | dni: %s | numero: %d", c.bet.DocNumber, c.bet.Number)
		}
	}
}

func (c *Client) Stop() {
	if c.conn != nil {
		c.conn.Close()
	}

	log.Infof("action: exit | result: success | client_id: %v", c.config.ID)
}

