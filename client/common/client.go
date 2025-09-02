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
	select {
	case <-sigChan:
		log.Infof("action: shutdown | result: success")
		return
	default:
		// Establecer conexión una sola vez
		if err := c.createClientSocket(); err != nil {
			return
		}
		defer c.conn.Close()

		c.processBets()
	}
}

// processBets procesa todas las apuestas enviándolas en batches usando una sola conexión
func (c *Client) processBets() {
	maxBatchSize := c.config.BatchMaxAmount
	totalBets := len(c.bets)

	if totalBets == 0 {
		log.Infof("action: apuestas_enviadas | result: success | client_id: %v", c.config.ID)
		return
	}

	log.Infof("action: starting_batch_processing | result: in_progress | client_id: %v | total_bets: %d | max_batch_size: %d",
		c.config.ID, totalBets, maxBatchSize)

	i := 0
	batchNumber := 1
	for i < totalBets {
		// Crear batch respetando tanto el límite de cantidad como el de 8KB
		batch := c.createBatch(i, maxBatchSize)

		if err := protocol.SendBetBatch(c.conn, batch); err != nil {
			log.Errorf("action: send_batch | result: fail | client_id: %v | batch_size: %d | error: %v",
				c.config.ID, len(batch), err)
			return
		}

		ok, err := protocol.ReceiveBatchAck(c.conn)
		if err != nil {
			log.Errorf("action: receive_batch_ack | result: fail | client_id: %v | error: %v", c.config.ID, err)
			return
		}

		if ok {
			log.Infof("action: batch_sent | result: success | client_id: %v | batch_number: %d | batch_size: %d | processed: %d/%d",
				c.config.ID, batchNumber, len(batch), i+len(batch), totalBets)
		} else {
			log.Errorf("action: batch_sent | result: fail | client_id: %v | batch_number: %d | batch_size: %d",
				c.config.ID, batchNumber, len(batch))
			return
		}

		i += len(batch)
		batchNumber++
	}

	log.Infof("action: all_bets_sent | result: success | client_id: %v | total_processed: %d", c.config.ID, totalBets)
}

// createBatch crea un batch respetando el límite de cantidad
func (c *Client) createBatch(start int, maxBatchSize int) []model.Bet {
	totalBets := len(c.bets)

	if start >= totalBets {
		return []model.Bet{}
	}

	var batch []model.Bet

	for i := start; i < totalBets && len(batch) < maxBatchSize; i++ {
		bet := c.bets[i]
		batch = append(batch, bet)
	}

	return batch
}

func (c *Client) Stop() {
	if c.conn != nil {
		c.conn.Close()
		log.Infof("action: close_connection | result: success | client_id: %v", c.config.ID)
	}

	log.Infof("action: exit | result: success | client_id: %v", c.config.ID)
}
