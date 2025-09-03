package common

import (
	"encoding/csv"
	"fmt"
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
	BatchMaxAmount int
}

type Client struct {
	config ClientConfig
	conn   net.Conn
}

func NewClient(config ClientConfig) *Client {
	return &Client{
		config: config,
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
		// Establecer conexión una sola vez para todo el proceso
		if err := c.createClientSocket(); err != nil {
			return
		}
		defer c.conn.Close()

		if err := c.processCSVFile(); err != nil {
			log.Errorf("action: process_csv | result: fail | client_id: %v | error: %v", c.config.ID, err)
			return
		}
		c.finishNotification()
		c.consultWinners()
	}
}

// processCSVFile lee el CSV y procesa las apuestas en batches sin cargar todo en memoria
func (c *Client) processCSVFile() error {
	filename := fmt.Sprintf("/agency-%s.csv", c.config.ID)

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("error opening CSV file %s: %w", filename, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	
	var batch []model.Bet
	maxBatchSize := c.config.BatchMaxAmount
	lineNumber := 0
	batchNumber := 1
	totalProcessed := 0
	
	log.Infof("action: starting_batch_processing | result: success | client_id: %v | max_batch_size: %d", c.config.ID, maxBatchSize)
	
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				// Enviar el último batch si tiene datos
				if len(batch) > 0 {
					if err := c.sendBatch(batch, batchNumber, totalProcessed); err != nil {
						return fmt.Errorf("error sending final batch: %w", err)
					}
					totalProcessed += len(batch)
				}
				break
			}
			return fmt.Errorf("error reading CSV file: %w", err)
		}
		
		lineNumber++
		if len(record) != 5 {
			return fmt.Errorf("invalid record in line %d: expected 5 fields, got %d", lineNumber, len(record))
		}

		bet := model.Bet{
			AgencyId:  c.config.ID,
			Name:      record[0],
			LastName:  record[1],
			Document:  record[2],
			BirthDate: record[3],
			Number:    record[4],
		}
		
		batch = append(batch, bet)
		
		// Cuando el batch alcanza el tamaño deseado, enviarlo
		if len(batch) >= maxBatchSize {
			if err := c.sendBatch(batch, batchNumber, totalProcessed); err != nil {
				return fmt.Errorf("error sending batch at line %d: %w", lineNumber, err)
			}
			totalProcessed += len(batch)
			batchNumber++
			// Limpiar el batch para liberar memoria
			batch = nil
		}
	}

	log.Infof("action: all_bets_sent | result: success | client_id: %v | total_processed: %d", c.config.ID, totalProcessed)
	return nil
}

// sendBatch envía un batch de apuestas al servidor
func (c *Client) sendBatch(batch []model.Bet, batchNumber int, totalProcessed int) error {
	log.Infof("action: sending_batch | result: success | batch_number: %d | batch_size: %d | client_id: %v",
		batchNumber, len(batch), c.config.ID)

	if err := protocol.SendBetBatch(c.conn, batch); err != nil {
		return fmt.Errorf("error sending batch: %w", err)
	}

	lastProcessedNumber, err := protocol.ReceiveBatchAck(c.conn)
	if err != nil {
		return fmt.Errorf("error receiving batch ack: %w", err)
	}

	// Obtener el número de la última apuesta que se envió en este batch
	expectedLastNumberStr := batch[len(batch)-1].Number
	expectedLastNumber, err := strconv.Atoi(expectedLastNumberStr)
	if err != nil {
		return fmt.Errorf("error parsing bet number %s: %w", expectedLastNumberStr, err)
	}
	
	if lastProcessedNumber <= 0 {
		return fmt.Errorf("no bets processed in batch")
	}

	// Verificar que el servidor procesó hasta donde esperábamos
	if lastProcessedNumber == expectedLastNumber {
		log.Infof("action: batch_sent | result: success | client_id: %v | batch_number: %d | batch_size: %d | last_processed_bet: %d | processed: %d",
			c.config.ID, batchNumber, len(batch), lastProcessedNumber, totalProcessed + len(batch))
	} else {
		return fmt.Errorf("server processed %d but expected %d", lastProcessedNumber, expectedLastNumber)
	}

	return nil
}

// finishNotification envía notificación al servidor de que terminó de enviar apuestas
func (c *Client) finishNotification() {
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

}

func (c *Client) Stop() {
	// Pequeño delay para evitar terminación simultánea
	clientID := c.config.ID

	if c.conn != nil {
		c.conn.Close()
		log.Infof("action: close_connection | result: success | client_id: %v", c.config.ID)
	}

	log.Infof("action: exit | result: success | client_id: %v", c.config.ID)
}
