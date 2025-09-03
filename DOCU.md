## EJ5
Modifiqué la lógica de cliente y servidor para emular el flujo de apuestas de una agencia de quiniela:

- **Cliente:** Cada cliente representa una agencia y recibe los datos de la apuesta (nombre, apellido, DNI, nacimiento, número) por variables de entorno. Estos datos se envían al servidor usando un protocolo propio basado en sockets y mensajes longitud-prefijados. Al recibir la confirmación del servidor, el cliente imprime en el log: `action: apuesta_enviada | result: success | dni: ${DNI} | numero: ${NUMERO}`.

- **Servidor:** El servidor recibe las apuestas de los clientes, las almacena usando la función provista `store_bet` y loguea cada registro con: `action: apuesta_almacenada | result: success | dni: ${DNI} | numero: ${NUMERO}`.

- **Comunicación:** Implementé un módulo de comunicación dedicado con separación clara de responsabilidades:
  - **Protocolo:** Header de 2 bytes big-endian con longitud del payload, seguido de campos separados por "|" (AgencyId|Nombre|Apellido|DNI|Fecha|Numero)
  - **ACK:** Respuesta de 4 bytes big-endian con el número de la apuesta confirmada

### Snippets del código:

#### Protocolo de Comunicación - Cliente (Go):
```go
// client/protocol/protocol.go
func SendBet(conn net.Conn, bet model.Bet) error {
    // Armamos el payload como string separado por "|"
    payload := fmt.Sprintf("%d|%s|%s|%s|%s|%d",
        bet.AgencyId, bet.Name, bet.LastName, 
        bet.Document, bet.BirthDate, bet.Number)

    data := []byte(payload)
    length := uint16(len(data))

    // Header de 2 bytes big-endian
    header := []byte{byte(length >> 8), byte(length & 0xFF)}

    // Enviamos header + payload
    if err := writeAll(conn, header); err != nil {
        return fmt.Errorf("error sending header: %w", err)
    }
    return writeAll(conn, data)
}

func ReceiveAck(conn net.Conn) (int, error) {
    buf := make([]byte, 4)
    if err := readAll(conn, buf); err != nil {
        return 0, fmt.Errorf("error reading ACK: %w", err)
    }
    // Reconstruimos el uint32 big-endian
    ackNumber := int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
    return ackNumber, nil
}
```

#### Protocolo de Comunicación - Servidor (Python):
```python
# server/protocol/protocol.py
def read_bet(sock) -> Bet:
    """Lee una apuesta usando longitud-prefijada (2 bytes) y separador '|'."""
    header = _read_n_bytes(sock, 2)
    message_length = (header[0] << 8) | header[1]
    data = _read_n_bytes(sock, message_length)
    text = data.decode('utf-8')

    fields = text.split('|')
    return Bet(
        agency=fields[0],
        first_name=fields[1], 
        last_name=fields[2],
        document=int(fields[3]),
        birthdate=fields[4],
        number=int(fields[5])
    )

def send_ack(sock, bet: Bet):
    """Envía un ACK de 4 bytes big-endian con el número de la apuesta."""
    n = bet.number
    ack = bytes([(n >> 24) & 0xFF, (n >> 16) & 0xFF, 
                 (n >> 8) & 0xFF, n & 0xFF])
    _send_all(sock, ack)
```

#### Manejo de Apuestas en el Servidor:
```python
# server/common/server.py
def __handle_client_connection(self, client_sock):
    try:
        bet = read_bet(client_sock)
        store_bets([bet])  # Función provista por la cátedra
        logging.info(f"action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}")
        send_ack(client_sock, bet)
    except Exception as e:
        logging.error(f"action: receive_message | result: fail | error: {e}")
    finally:
        client_sock.close()
```

## EJ4
Tanto el servidor como el cliente fueron modificados para implementar un shutdown graceful al recibir la señal SIGTERM:

- **Servidor:** Se utiliza `signal.signal(signal.SIGTERM, self.handle_sigterm)` para capturar la señal. Al recibirla, el servidor deja de aceptar nuevas conexiones (`self.running = False`), cierra todas las conexiones pendientes mediante `close_pending_connections()`, y finalmente cierra el socket principal (`self._server_socket.close()`).

- **Cliente:** Al recibir la señal, se ejecuta el método `Stop()`, que cierra la conexión activa (`c.conn.Close()`) y loguea el cierre y la salida del cliente. Además, en el ciclo principal (`StartClientLoop`) se verifica la señal y se interrumpe el envío de mensajes si corresponde, asegurando que no queden recursos abiertos.

### Snippets importantes del código:

#### Manejo de SIGTERM - Servidor (Python):
```python
# server/common/server.py
def __init__(self, port, listen_backlog):
    self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    self._server_socket.bind(('', port))
    self._server_socket.listen(listen_backlog)
    self.running = True
    signal.signal(signal.SIGTERM, self.handle_sigterm)

def handle_sigterm(self, signum, frame):
    """Handle SIGTERM signal for graceful shutdown"""
    logging.info('action: shutdown | result: in_progress')
    self.running = False
    
    try:
        self._server_socket.close()
        logging.info('action: server_socket_closed | result: success')
    except Exception as e:
        logging.error(f'action: server_socket_closed | result: fail | error: {e}')
    
    logging.info('action: shutdown | result: success')
```

#### Manejo de SIGTERM - Cliente (Go):
```go
// client/main.go
func main() {
    sigchan := make(chan os.Signal, 1)
    signal.Notify(sigchan, syscall.SIGTERM)
    
    client := common.NewClient(clientConfig, bet)
    
    go func() {
        <-sigchan
        client.Stop()
    }()
    
    client.StartClientLoop(sigchan)
}

// client/common/client.go
func (c *Client) StartClientLoop(sigChan chan os.Signal) {
    select {
    case <-sigChan:
        log.Infof("action: shutdown | result: success")
        return
    default:
        // ... lógica de conexión y envío ...
    }
}

func (c *Client) Stop() {
    if c.conn != nil {
        c.conn.Close()
        log.Infof("action: close_connection | result: success | client_id: %v", c.config.ID)
    }
    log.Infof("action: exit | result: success | client_id: %v", c.config.ID)
}
```

## EJ3
Implementé el script `validar-echo-server.sh` en la raíz del proyecto para verificar automáticamente el funcionamiento del servidor echo. El script utiliza netcat dentro de un contenedor BusyBox conectado a la red interna de Docker, envía un mensaje al servidor y espera recibir exactamente el mismo mensaje como respuesta. Si la validación es exitosa, imprime `action: test_echo_server | result: success`, y si falla, imprime `action: test_echo_server | result: fail`. De este modo, no es necesario instalar netcat en la máquina host ni exponer puertos del servidor.

## EJ2
Modifiqué el cliente y el servidor para que lean su configuración desde archivos externos (`config.yaml` para el cliente y `config.ini` para el servidor), que se montan como volúmenes en los contenedores Docker. Así, cualquier cambio en la configuración se aplica sin necesidad de reconstruir las imágenes, solo actualizando los archivos en el host.

## EJ1
Implementé el script `generar-compose.sh` en la raíz del proyecto, que genera un archivo Docker Compose con la cantidad de clientes indicada por parámetro (ej: `./generar-compose.sh docker-compose-dev.yaml 5`).
Valida los parámetros, escribe la cabecera, agrega los servicios `client1`, `client2`, etc., y la red al final.