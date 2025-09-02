import signal
import socket
import logging
import threading
from concurrent.futures import ThreadPoolExecutor

from common.utils import store_bets, load_bets, has_won
from protocol.protocol import read_message, send_winners_list, parse_bet_batch_content, send_batch_ack, send_simple_ack

class Server:
    def __init__(self, port, listen_backlog, expected_agencies):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        
        # Variables de control del servidor
        self.running = True
        self.client_connections = []
        self.finishedAgencies = 0  # Contador de agencias que terminaron de enviar apuestas
        self.expected_agencies = expected_agencies  # Cuántas agencias espero en total
        self.sorteoRealizado = False  # Flag para saber si ya se hizo el sorteo
        
        # Lock principal para proteger variables compartidas entre threads
        self.lock = threading.Lock()
        
        # Lock separado para las funciones de archivo (store_bets, load_bets)
        # Esto evita que dos threads escriban/lean el CSV al mismo tiempo
        self.file_lock = threading.Lock()
        
        # Pool de threads para atender múltiples clientes a la vez
        # Máximo 10 para no saturar el sistema
        self.thread_pool = ThreadPoolExecutor(max_workers=10)

        # Configuro el manejo de SIGTERM para cierre limpio
        signal.signal(signal.SIGTERM, self.handle_sigterm)
        
        logging.info(f"action: config | result: success | expected_agencies: {expected_agencies}")

    def run(self):
        # Loop principal del servidor - acepta conexiones y las delega a threads
        while self.running:
            try:
                client_sock, addr = self.__accept_new_connection()
                if client_sock:
                    # Delego cada cliente a un thread del pool para atenderlo concurrentemente
                    # Así puedo atender múltiples clientes al mismo tiempo
                    self.thread_pool.submit(self.__handle_client_connection, client_sock)
            except OSError:
                if self.running:
                    logging.error("action: accept_connections | result: fail | error: Socket error")
                break

    def handle_sigterm(self, signum, frame):
        """
        Manejo del shutdown graceful cuando llega SIGTERM
        """
        self.running = False
        
        # Primero cierro todas las conexiones que están esperando ganadores
        self.close_pending_connections()
        
        # Cierro el socket principal del servidor
        try:
            self._server_socket.close()
            logging.info('action: server_socket_closed | result: success')
        except Exception as e:
            logging.error(f'action: server_socket_closed | result: fail | error: {e}')
        
        # Espero a que terminen todos los threads antes de salir
        # wait=True hace que espere hasta que todos los workers terminen
        try:
            self.thread_pool.shutdown(wait=True)
        except Exception as e:
            logging.error(f'action: thread_pool_shutdown | result: fail | error: {e}')
        
        logging.info('action: shutdown | result: success')

    def __handle_client_connection(self, client_sock):
        """
        Atiende un cliente completo en un thread separado.
        Cada cliente sigue este flujo:
        1. Envía varios BATCH_APUESTAS (las apuestas en grupos)
        2. Envía FIN_APUESTAS (avisa que terminó)
        3. Envía CONSULTA_GANADORES (pide los ganadores de su agencia)
        """
        
        try:
            # Un cliente mantiene la conexión abierta y envía varios mensajes
            while True:
                # Leo el próximo mensaje y veo qué tipo es
                msg_type, content = read_message(client_sock)

                if msg_type == 'BATCH_APUESTAS':
                    # Es un grupo de apuestas, las parseo
                    bets = parse_bet_batch_content(content)

                    last_processed_bet_number = 0
                    try:
                        # uso file_lock para que solo un thread escriba al CSV
                        with self.file_lock:
                            store_bets(bets)
                        
                        # Guardo el número de la última apuesta procesada
                        for bet in bets:
                            last_processed_bet_number = bet.number
                        
                        logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")
                    except Exception as e:
                        logging.error(f"action: store_bets | result: fail | error: {e}")
                        logging.info(f"action: apuesta_recibida | result: fail | cantidad: {len(bets)}")
                    
                    # Le confirmo al cliente qué apuestas procesé bien
                    send_batch_ack(client_sock, last_processed_bet_number)
                
                elif msg_type == 'FIN_APUESTAS':
                    # El cliente me avisa que terminó de enviar todas sus apuestas
                    agency_id = content
                    
                    # Uso lock porque varios threads pueden llegar acá al mismo tiempo
                    with self.lock:
                        self.finishedAgencies += 1
                        # Si ya terminaron todas las agencias, hago el sorteo
                        if self.finishedAgencies == self.expected_agencies and not self.sorteoRealizado:
                            self.sorteoRealizado = True
                            logging.info("action: sorteo | result: success")
                            # Respondo a todos los clientes que estaban esperando ganadores
                            self.respond_pending_winners()      
                    
                    # Le confirmo que recibí su notificación
                    send_simple_ack(client_sock, True)
                    
                elif msg_type == 'CONSULTA_GANADORES':
                    # El cliente pide los ganadores de su agencia
                    agency_id = content
                    
                    if not self.sorteoRealizado:
                        # El sorteo aún no se hizo, pongo al cliente en lista de espera
                        # NO cierro la conexión
                        with self.lock:
                            # Agregar cliente a la lista de espera (no cierra la conexion)
                            self.client_connections.append((client_sock, int(agency_id)))
                        return
                    else:
                        # Ya se hizo el sorteo, busco los ganadores y respondo
                        winners = self.get_winners_agency(int(agency_id))
                        send_winners_list(client_sock, winners, sorteoRealizado=True)
                        break

        except Exception as e:
            # Si hay error, saco al cliente de la lista de espera
            self.remove_from_pending(client_sock)
        finally:
            # Solo cierro la conexión si el cliente no está esperando ganadores
            if not self.is_connection_pending(client_sock):
                try:
                    client_sock.close()
                    logging.info('action: client_socket_closed | result: success')
                except Exception as e:
                    logging.error(f"action: client_socket_close | result: fail | error: {e}")

    def get_winners_agency(self, agency_id: int) -> list[str]:
        """
        Busca los ganadores de una agencia específica
        """
        try:
            # Uso file_lock para leer el CSV de forma segura
            # Así no leo mientras otro thread está escribiendo
            with self.file_lock:
                bets = list(load_bets())
            
            # Filtro solo los ganadores de esta agencia
            winners_dni = []
            for bet in bets:
                if bet.agency == agency_id and has_won(bet):
                    winners_dni.append(bet.document)
            
            return winners_dni
        except Exception as e:
            return []

    def respond_pending_winners(self):
        """
        Responde a todos los clientes que estaban esperando los resultados del sorteo
        """
        try:
            for client_sock, agency_id in self.client_connections:
                try:
                    winners = self.get_winners_agency(agency_id)
                    send_winners_list(client_sock, winners, sorteoRealizado=True)
                    logging.info(
                        f"action: ganadores_enviados | result: success "
                        f"| cant_ganadores: {len(winners)} | agency: {agency_id}"
                    )
                    client_sock.close()
                except Exception as e:
                    logging.error(
                        f"action: respond_pending_winners | result: fail "
                        f"| agency: {agency_id} | error: {e}"
                    )
                    try:
                        client_sock.close()
                    except:
                        pass
        finally:
            self.client_connections.clear()

    def close_pending_connections(self):
        """
        Cierra todas las conexiones pendientes de consultas de ganadores.
        Se llama durante el shutdown para evitar conexiones abiertas.
        """
        with self.lock:
            pendientes = list(self.client_connections)
            self.client_connections.clear()
        logging.info(f"action: close_pending_connections | result: in_progress | count: {len(pendientes)}")
        for client_sock, agency_id in pendientes:
            try:
                client_sock.close()
                logging.info(f'action: pending_connection_closed | result: success | agency: {agency_id}')
            except Exception as e:
                logging.error(f"action: close_pending_connection | result: fail | agency: {agency_id} | error: {e}")

    def remove_from_pending(self, client_sock):
        """
        Remueve una conexión específica de la lista de pendientes.
        """
        with self.lock:
            self.client_connections = [
                (sock, agency_id) for sock, agency_id in self.client_connections
                if sock != client_sock
            ]

    def is_connection_pending(self, client_sock):
        """
        Verifica si una conexión está en la lista de pendientes.
        """
        with self.lock:
            return any(sock == client_sock for sock, _ in self.client_connections)

    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned
        """
        
        if not self.running:
            return None, None

        # Connection llega
        logging.info('action: accept_connections | result: in_progress')
        try:
            c, addr = self._server_socket.accept()
            logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
            return c, addr
        except OSError:
            if self.running:
                logging.error('action: accept_connections | result: fail | error: Socket closed')
            return None, None