package common

import (
	"net"
	"os"
	"strings"
	"time"

	"github.com/7574-sistemas-distribuidos/docker-compose-init/client/model"
	"github.com/7574-sistemas-distribuidos/docker-compose-init/client/protocol"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

type ClientConfig struct {
	ID             string
	ServerAddress  string
	LoopAmount     int
	LoopPeriod     time.Duration
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
	conn, err := net.Dial("tcp", c.config.ServerAddress)
	if err != nil {
		log.Errorf("action: connect | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return err
	}
	c.conn = conn
	return nil
}

// StartClient inicia el cliente y maneja la señal de apagado
func (c *Client) StartClientLoop(sigChan chan os.Signal) {
	select {
	case <-sigChan:
		log.Infof("action: shutdown | result: success")
		return
	default:
		c.processBets()
	}
}

// processBets procesa todas las apuestas enviándolas en batches
func (c *Client) processBets() {
	batchSize := c.config.BatchMaxAmount
	totalBets := len(c.bets)

	if totalBets == 0 {
		log.Infof("action: apuestas_enviadas | result: success | client_id: %v", c.config.ID)
		return
	}

	if batchSize <= 0 {
		batchSize = totalBets
	}

	for i := 0; i < totalBets; i += batchSize {
		end := i + batchSize
		if end > totalBets {
			end = totalBets
		}

		batch := c.bets[i:end]
		if err := c.createClientSocket(); err != nil {
			return
		}

		if err := protocol.SendBetBatch(c.conn, batch); err != nil {
			log.Errorf("action: send_batch | result: fail | client_id: %v | batch_size: %d | error: %v",
				c.config.ID, len(batch), err)
			c.conn.Close()
			return
		}

		ok, err := protocol.ReceiveBatchAck(c.conn)
		if err != nil {
			log.Errorf("action: receive_batch_ack | result: fail | client_id: %v | error: %v", c.config.ID, err)
			c.conn.Close()
			return
		}

		// Dar tiempo al servidor para procesar antes de cerrar
		time.Sleep(10 * time.Millisecond)
		c.conn.Close()

		if ok {
			log.Infof("action: apuesta_enviada | result: success | cantidad: %d", len(batch))
		} else {
			log.Errorf("action: apuesta_enviada | result: fail | client_id: %v | batch_size: %d", c.config.ID, len(batch))
			return
		}
	}

	log.Infof("action: apuesta_enviada | result: success | client_id: %v", c.config.ID)

	// Enviar notificación de finalización
	c.finishNotification()

	// Consultar ganadores
	c.consultWinners()
}

// sendFinishNotification envía notificación al servidor de que terminó de enviar apuestas
func (c *Client) finishNotification() {
	if err := c.createClientSocket(); err != nil {
		log.Errorf("action: send_finish_notification | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return
	}
	defer c.conn.Close()

	if err := protocol.SendFinishConfirmation(c.conn, c.config.ID); err != nil {
		log.Errorf("action: send_finish_notification | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return
	}

	ok, err := protocol.ReceiveFinishAck(c.conn)
	if err != nil {
		log.Errorf("action: receive_finish_ack | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return
	}

	if ok {
		log.Infof("action: finish_notification | result: success | client_id: %v", c.config.ID)
	} else {
		log.Errorf("action: finish_notification | result: fail | client_id: %v", c.config.ID)
	}
}

// consultWinners consulta la lista de ganadores al servidor
func (c *Client) consultWinners() {
	maxRetries := 10              // Más intentos
	retryDelay := 3 * time.Second // Más tiempo entre intentos

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := c.createClientSocket(); err != nil {
			log.Errorf("action: winners | result: fail | client_id: %v | attempt: %d | error: %v",
				c.config.ID, attempt, err)
			if attempt < maxRetries {
				time.Sleep(retryDelay)
				continue
			}
			return
		}

		if err := protocol.SendWinnersQuery(c.conn, c.config.ID); err != nil {
			log.Errorf("action: ask_winners | result: fail | client_id: %v | attempt: %d | error: %v",
				c.config.ID, attempt, err)
			c.conn.Close()
			if attempt < maxRetries {
				time.Sleep(retryDelay)
				continue
			}
			return
		}

		winners, err := protocol.ReceiveWinnersList(c.conn)
		c.conn.Close()

		if err != nil {
			if strings.Contains(err.Error(), "ERROR_NO_SORTEO") {
				// No loggear como error, solo como info que está esperando
				log.Infof("action: consulta_ganadores | result: waiting | client_id: %v | attempt: %d | reason: sorteo_pending",
					c.config.ID, attempt)
				if attempt < maxRetries {
					time.Sleep(retryDelay)
					continue
				}
				// Solo loggear error si se agotan todos los intentos
				log.Errorf("action: consulta_ganadores | result: fail | client_id: %v | reason: max_retries_exceeded", c.config.ID)
				return
			}
			log.Errorf("action: query_winners | result: fail | client_id: %v | attempt: %d | error: %v",
				c.config.ID, attempt, err)
			return
		}

		// Éxito - log requerido por el enunciado
		log.Infof("action: consulta_ganadores | result: success | cant_ganadores: %d", len(winners))

		if len(winners) > 0 {
			log.Infof("action: ganadores_recibidos | result: success | client_id: %v | ganadores: %v",
				c.config.ID, winners)
		}
		return
	}
}

func (c *Client) Stop() {
	if c.conn != nil {
		c.conn.Close()
		log.Infof("action: close_connection | result: success | client_id: %v", c.config.ID)
	}

	log.Infof("action: exit | result: success | client_id: %v", c.config.ID)
}
