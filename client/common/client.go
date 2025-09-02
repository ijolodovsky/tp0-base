package common

import (
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

// Función principal del cliente - maneja todo el flujo
func (c *Client) StartClientLoop(sigChan chan os.Signal) {
	// Primero verifico si llegó una señal de cierre
	select {
	case <-sigChan:
		log.Infof("action: shutdown | result: success")
		return
	default:
		// Creo una unica conexión para todo el proceso
		if err := c.createClientSocket(); err != nil {
			return
		}
		// Aseguro que se cierre la conexión al final
		defer c.conn.Close()

		// Flujo completo del cliente:
		c.processBets()      // 1. Envío todas las apuestas en batches
		c.finishNotification() // 2. Aviso que terminé
		c.consultWinners()     // 3. Consulto ganadores
	}
}

// Envío todas las apuestas al servidor en grupos (batches)
func (c *Client) processBets() {
	maxBatchSize := c.config.BatchMaxAmount
	totalBets := len(c.bets)

	// Si no tengo apuestas, termino
	if totalBets == 0 {
		log.Infof("action: apuestas_enviadas | result: success | client_id: %v", c.config.ID)
		return
	}

	log.Infof("action: starting_batch_processing | result: success | client_id: %v | total_bets: %d | max_batch_size: %d",
		c.config.ID, totalBets, maxBatchSize)

	i := 0
	batchNumber := 1
	for i < totalBets {
		// Crear batch respetando el límite de cantidad
		batch := c.createBatch(i, maxBatchSize)

		log.Infof("action: sending_batch | result: success | batch_number: %d | batch_size: %d | client_id: %v",
			batchNumber, len(batch), c.config.ID)

		// Envío el batch al servidor
		if err := protocol.SendBetBatch(c.conn, batch); err != nil {
			log.Errorf("action: send_batch | result: fail | client_id: %v | batch_size: %d | error: %v",
				c.config.ID, len(batch), err)
			return
		}

		// Espero confirmación del servidor
		lastProcessedNumber, err := protocol.ReceiveBatchAck(c.conn)
		if err != nil {
			log.Errorf("action: receive_batch_ack | result: fail | client_id: %v | error: %v", c.config.ID, err)
			return
		}

		// Verifico que el servidor procesó bien todas las apuestas
		expectedLastNumberStr := batch[len(batch)-1].Number
		expectedLastNumber, err := strconv.Atoi(expectedLastNumberStr)
		if err != nil {
			log.Errorf("action: parse_bet_number | result: fail | client_id: %v | bet_number: %s | error: %v", 
				c.config.ID, expectedLastNumberStr, err)
			return
		}
		
		if lastProcessedNumber > 0 {
			// El servidor me dice hasta qué apuesta procesó
			if lastProcessedNumber == expectedLastNumber {
				log.Infof("action: batch_sent | result: success | client_id: %v | batch_number: %d | batch_size: %d | last_processed_bet: %d | processed: %d/%d",
					c.config.ID, batchNumber, len(batch), lastProcessedNumber, i+len(batch), totalBets)
			} else {
				log.Errorf("action: batch_sent | result: partial_success | client_id: %v | batch_number: %d | batch_size: %d | expected_last: %d | actual_last: %d | processed: %d/%d",
					c.config.ID, batchNumber, len(batch), expectedLastNumber, lastProcessedNumber, i+len(batch), totalBets)
				return
			}
		} else {
			log.Errorf("action: batch_sent | result: fail | client_id: %v | batch_number: %d | batch_size: %d | no_bets_processed",
				c.config.ID, batchNumber, len(batch))
			return
		}

		// Avanzo al siguiente grupo de apuestas
		i += len(batch)
		batchNumber++
	}

	log.Infof("action: all_bets_sent | result: success | client_id: %v | total_processed: %d", c.config.ID, totalBets)
}

// createBatch crea un batch respetando el límite de cantidad
func (c *Client) createBatch(start int, maxBatchSize int) []model.Bet {
	totalBets := len(c.bets)

	// Si ya procesé todas, devuelvo lista vacía
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

// finishNotification envía notificación al servidor de que terminó de enviar apuestas
func (c *Client) finishNotification() {
	if err := protocol.SendFinishConfirmation(c.conn, c.config.ID); err != nil {
		log.Errorf("action: send_finish_notification | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return
	}

	// Espero confirmación del servidor
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
	if err := protocol.SendWinnersQuery(c.conn, c.config.ID); err != nil {
		log.Errorf("action: ask_winners | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return
	}

	// Esperar respuesta (el servidor mantiene la conexión hasta tener los resultados)
	winners, err := protocol.ReceiveWinnersList(c.conn)
	if err != nil {
		log.Errorf("action: ask_winners | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return
	}

	log.Infof("action: consulta_ganadores | result: success | cant_ganadores: %d", len(winners))

	if len(winners) > 0 {
		log.Infof("action: ganadores_recibidos | result: success | client_id: %v | ganadores: %v",
			c.config.ID, winners)
	}

	// Delay para asegurar que los logs se escriban antes de terminar
	time.Sleep(100 * time.Millisecond)
}

// Cierre limpio del cliente cuando llega SIGTERM
func (c *Client) Stop() {
	// Pequeño delay escalonado para evitar que todos cierren a la vez
	clientID := c.config.ID
	if len(clientID) > 0 {
		lastChar := clientID[len(clientID)-1]
		delay := time.Duration(int(lastChar-'0')) * 50 * time.Millisecond
		time.Sleep(delay)
	}

	// Cierro la conexión si está abierta
	if c.conn != nil {
		c.conn.Close()
		log.Infof("action: close_connection | result: success | client_id: %v", c.config.ID)
	}

	log.Infof("action: exit | result: success | client_id: %v", c.config.ID)
}
