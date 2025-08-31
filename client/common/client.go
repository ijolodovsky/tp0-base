package common

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/op/go-logging"
	"github.com/7574-sistemas-distribuidos/docker-compose-init/client/model"
	"github.com/7574-sistemas-distribuidos/docker-compose-init/client/protocol"
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
	
	for i := 0; i < totalBets; i += batchSize {
		end := i + batchSize
		if end > totalBets {
			end = totalBets
		}
		
		batch := c.bets[i:end]
		if err := c.sendBatch(batch); err != nil {
			log.Errorf("action: send_batch | result: fail | client_id: %v | batch_size: %d | error: %v", 
				c.config.ID, len(batch), err)
			return
		}
		
		log.Infof("action: batch_processed | result: success | client_id: %v | batch_size: %d", 
			c.config.ID, len(batch))
	}
}

// sendBatch envía un batch de apuestas al servidor
func (c *Client) sendBatch(batch []model.Bet) error {
	if err := c.createClientSocket(); err != nil {
		return err
	}
	defer c.conn.Close()

	// Verificar que el tamaño del batch no exceda 8kB aproximadamente
	estimatedSize := c.estimateBatchSize(batch)
	if estimatedSize > 8000 { // 8kB
		log.Warnf("Batch size (%d bytes) exceeds 8kB limit", estimatedSize)
	}

	// Enviar batch de apuestas
	if err := protocol.SendBetBatch(c.conn, batch); err != nil {
		return fmt.Errorf("error sending batch: %w", err)
	}

	// Recibir respuesta del servidor
	success, err := protocol.ReceiveBatchAck(c.conn)
	if err != nil {
		return fmt.Errorf("error receiving batch ACK: %w", err)
	}

	if success {
		for _, bet := range batch {
			log.Infof("action: apuesta_enviada | result: success | dni: %s | numero: %d", bet.Document, bet.Number)
		}
	} else {
		for _, bet := range batch {
			log.Errorf("action: apuesta_enviada | result: fail | dni: %s | numero: %d", bet.Document, bet.Number)
		}
		return fmt.Errorf("server reported batch processing failed")
	}

	return nil
}

// estimateBatchSize estima el tamaño en bytes de un batch
func (c *Client) estimateBatchSize(batch []model.Bet) int {
	size := 2 // 2 bytes para el header
	for _, bet := range batch {
		// Estimamos el tamaño de cada apuesta
		betSize := len(fmt.Sprintf("%d|%s|%s|%s|%s|%d",
			bet.AgencyId, bet.Name, bet.LastName, bet.Document, bet.BirthDate, bet.Number))
		size += betSize + 1 // +1 para el '\n'
	}
	return size
}

func (c *Client) Stop() {
	if c.conn != nil {
		c.conn.Close()
		log.Infof("action: close_connection | result: success | client_id: %v", c.config.ID)
	}

	log.Infof("action: exit | result: success | client_id: %v", c.config.ID)
}

