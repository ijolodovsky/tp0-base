package common

import (
	"io"
	"net"
	"os"
	"strconv"
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

	if totalBets == 0 {
		log.Infof("action: apuestas_enviadas | result: success | client_id: %v", c.config.ID)
		return
	}

	if batchSize <= 0 {
		c.config.BatchMaxAmount = totalBets
	}

	if err := c.createClientSocket(); err != nil {
		return
	}

	defer c.conn.Close()

	for i := 0; i < totalBets; i += batchSize {
		end := i + batchSize
		if end > totalBets {
			end = totalBets
		}
		
		batch := c.bets[i:end]
		if err := protocol.SendBetBatch(c.conn, batch); err != nil {
			log.Errorf("action: send_batch | result: fail | client_id: %v | batch_size: %d | error: %v",
				c.config.ID, len(batch), err)
			return
		}

		ack, err := protocol.ReceiveAck(c.conn)
		lastBet := batch[len(batch)-1]
		if err != nil {
			if err == io.EOF && end == totalBets{
				log.Infof("action: apuestas_enviadas | result: success | client_id: %v | last_bet: %v",
					c.config.ID, lastBet)
			}
			log.Errorf("action: receive_ack | result: fail | client_id: %v | last_bet: %v | error: %v",
				c.config.ID, lastBet, err)
			return
		}

		log.Infof("action: apuestas_enviadas | result: success | client_id: %v | batch_size: %d",
			c.config.ID, len(batch))

			num, err := strconv.Atoi(lastBet.Number)
			if err == nil && num == ack{
				log.Errorf("action: apuestas_enviadas | result: success | client_id: %v | last_bet: %v | ack_number: %d",
					c.config.ID, lastBet, ack)
			} else{
				log.Errorf("action: apuestas_enviadas | result: fail | client_id: %v | last_bet: %v | error: %v",
					c.config.ID, lastBet, err)
			}

		}
	

	log.Infof("action: apuestas_enviadas | result: success | client_id: %v", c.config.ID)
}

func (c *Client) Stop() {
	if c.conn != nil {
		c.conn.Close()
		log.Infof("action: close_connection | result: success | client_id: %v", c.config.ID)
	}

	log.Infof("action: exit | result: success | client_id: %v", c.config.ID)
}