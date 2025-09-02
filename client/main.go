package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/op/go-logging"
	"github.com/spf13/viper"

	"github.com/7574-sistemas-distribuidos/docker-compose-init/client/common"
	"github.com/7574-sistemas-distribuidos/docker-compose-init/client/model"
)

var log = logging.MustGetLogger("log")

// Cargo todas las apuestas desde el archivo CSV de mi agencia
func LoadBetsFromCSV(agencyId string) ([]model.Bet, error) {
	// Cada agencia tiene su archivo: /agency-1.csv, /agency-2.csv, etc.
	// Estos archivos se montan como volumen desde .data/
	filename := fmt.Sprintf("/agency-%s.csv", agencyId)

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening CSV file %s: %w", filename, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV file: %w", err)
	}

	// Convierto cada fila en una apuesta
	var bets []model.Bet
	for i, record := range records {
		// Cada fila debe tener 5 campos: nombre,apellido,dni,fecha,numero
		if len(record) != 5 {
			return nil, fmt.Errorf("invalid record in line %d: expected 5 fields, got %d", i+1, len(record))
		}

		// Creo la apuesta agregando el ID de mi agencia
		bet := model.Bet{
			AgencyId:  agencyId,
			Name:      record[0],
			LastName:  record[1],
			Document:  record[2],
			BirthDate: record[3],
			Number:    record[4],
		}
		bets = append(bets, bet)
	}

	return bets, nil
}

// InitConfig Function that uses viper library to parse configuration parameters.
// Viper is configured to read variables from both environment variables and the
// config file ./config.yaml. Environment variables takes precedence over parameters
// defined in the configuration file. If some of the variables cannot be parsed,
// an error is returned
func InitConfig() (*viper.Viper, error) {
	v := viper.New()

	// Configure viper to read env variables with the CLI_ prefix
	v.AutomaticEnv()
	v.SetEnvPrefix("cli")
	// Use a replacer to replace env variables underscores with points. This let us
	// use nested configurations in the config file and at the same time define
	// env variables for the nested configurations
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Add env variables supported
	v.BindEnv("id")
	v.BindEnv("server", "address")
	v.BindEnv("log", "level")
	v.BindEnv("batch", "maxAmount")

	// Try to read configuration from config file. If config file
	// does not exists then ReadInConfig will fail but configuration
	// can be loaded from the environment variables so we shouldn't
	// return an error in that case
	v.SetConfigFile("./config.yaml")
	if err := v.ReadInConfig(); err != nil {
		fmt.Printf("Configuration could not be read from config file. Using env variables instead")
	}

	return v, nil
}

// InitLogger Receives the log level to be set in go-logging as a string. This method
// parses the string and set the level to the logger. If the level string is not
// valid an error is returned
func InitLogger(logLevel string) error {
	baseBackend := logging.NewLogBackend(os.Stdout, "", 0)
	format := logging.MustStringFormatter(
		`%{time:2006-01-02 15:04:05} %{level:.5s}     %{message}`,
	)
	backendFormatter := logging.NewBackendFormatter(baseBackend, format)

	backendLeveled := logging.AddModuleLevel(backendFormatter)
	logLevelCode, err := logging.LogLevel(logLevel)
	if err != nil {
		return err
	}
	backendLeveled.SetLevel(logLevelCode, "")

	// Set the backends to be used.
	logging.SetBackend(backendLeveled)
	return nil
}

// PrintConfig Print all the configuration parameters of the program.
// For debugging purposes only
func PrintConfig(v *viper.Viper) {
	log.Infof("action: config | result: success | client_id: %s | server_address: %s | batch_max_amount: %v | log_level: %s",
		v.GetString("id"),
		v.GetString("server.address"),
		v.GetInt("batch.maxAmount"),
		v.GetString("log.level"),
	)
}

func main() {
	if err := run(); err != nil {
		log.Criticalf("Error fatal: %s", err)
		return
	}
}

func run() error {
	v, err := InitConfig()
	if err != nil {
		return fmt.Errorf("error inicializando configuración: %w", err)
	}

	if err := InitLogger(v.GetString("log.level")); err != nil {
		return fmt.Errorf("error inicializando logger: %w", err)
	}

	// Cargar apuestas desde CSV
	bets, err := LoadBetsFromCSV(v.GetString("id"))
	if err != nil {
		return fmt.Errorf("error cargando apuestas desde CSV: %w", err)
	}

	log.Infof("Cargadas %d apuestas desde CSV para agencia %s", len(bets), v.GetString("id"))

	// Configuro el manejo de señales para shutdown graceful
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGTERM)

	// Print program config with debugging purposes
	PrintConfig(v)

	clientConfig := common.ClientConfig{
		ServerAddress:  v.GetString("server.address"),
		ID:             v.GetString("id"),
		BatchMaxAmount: v.GetInt("batch.maxAmount"),
	}

	client := common.NewClient(clientConfig, bets)

	// 7. Configuro goroutine para manejar SIGTERM
	go func() {
		<-sigchan
		client.Stop()
	}()

	client.StartClientLoop(sigchan)

	log.Infof("action: exit | result: success | client_id: %v", v.GetString("id"))
	return nil
}
