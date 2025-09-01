import signal
import socket
import logging
import threading

from common.utils import store_bets, load_bets, has_won
from protocol.protocol import read_message, send_batch_ack, send_finish_ack, send_winners_list, parse_bet_batch_content

class Server:
    def __init__(self, port, listen_backlog, expected_agencies):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self.running = True
        self.client_connections = []
        self.finishedAgencies = 0
        self.expected_agencies = expected_agencies  # Cantidad esperada de agencias
        self.sorteoRealizado = False
        self.pending_winners_queries = []  # Lista de (socket, agency_id) esperando ganadores
        self.lock = threading.Lock()

        signal.signal(signal.SIGTERM, self.handle_sigterm)
        
        # Log de configuración
        logging.info(f"action: config | result: success | expected_agencies: {expected_agencies}")

    def run(self):
        while self.running:
            try:
                hilo = threading.Thread(target=self.__accept_client)
                hilo.start()
            except OSError:
                break

    def __accept_client(self):
        client_sock, addr = self._server_socket.accept()
        self.__handle_client_connection(client_sock)

    def handle_sigterm(self, signum, frame):
        self.running = False
        
        # Cerrar todas las conexiones pendientes de consultas de ganadores
        self.close_pending_connections()
        
        # Cerrar el socket del servidor
        self._server_socket.close()

    def __handle_client_connection(self, client_sock):
        """
        Handle multiple messages from a client connection in sequence:
        1. Multiple BATCH_APUESTAS messages
        2. One FIN_APUESTAS message
        3. One CONSULTA_GANADORES message
        """
        
        try:
            # manejar multiples mensajes en una sola conexion
            while True:
                msg_type, content = read_message(client_sock)

                if msg_type == 'BATCH_APUESTAS':
                    bets = parse_bet_batch_content(content)

                    all_success = True
                    try:
                        store_bets(bets)
                    except Exception as e:
                        all_success = False
                    
                    # Logging segun resultado del batch
                    if all_success:
                        logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")
                    else:
                        logging.info(f"action: apuesta_recibida | result: fail | cantidad: {len(bets)}")
                    
                    send_batch_ack(client_sock, all_success)
                
                elif msg_type == 'FIN_APUESTAS':
                    agency_id = content
                    with self.lock:
                        self.finishedAgencies += 1
                        # mira si ya terminaron todas las agencias esperadas
                        if self.finishedAgencies == self.expected_agencies and not self.sorteoRealizado:
                            self.sorteoRealizado = True
                            logging.info("action: sorteo | result: success")
                            # Responder a todos los clientes que estaban esperando ganadores
                            self.respond_pending_winners()      
                    send_finish_ack(client_sock, True)
                    
                elif msg_type == 'CONSULTA_GANADORES':
                    agency_id = content
                    
                    if not self.sorteoRealizado:
                        with self.lock:
                            # Agregar cliente a la lista de espera (no cierra la conexion)
                            self.pending_winners_queries.append((client_sock, int(agency_id)))
                        return
                    else:
                        # Busco los ganadores de esta agencia
                        winners = self.get_winners_agency(int(agency_id))
                        send_winners_list(client_sock, winners, sorteoRealizado=True)
                        break

        except Exception as e:
            self.remove_from_pending(client_sock)
        finally:
            if not self.is_connection_pending(client_sock):
                try:
                    client_sock.close()
                except:
                    pass

    def get_winners_agency(self, agency_id: int) -> list[str]:
        """
        Obtiene los DNI de los ganadores de la agencia especifica.
        """
        try:
            bets = list(load_bets())
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
            for client_sock, agency_id in self.pending_winners_queries:
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
            self.pending_winners_queries.clear()

    def close_pending_connections(self):
        """
        Cierra todas las conexiones pendientes de consultas de ganadores.
        Se llama durante el shutdown para evitar conexiones abiertas.
        """
        with self.lock:
            pendientes = list(self.pending_winners_queries)
            self.pending_winners_queries.clear()
        logging.info(f"action: close_pending_connections | result: in_progress | count: {len(pendientes)}")
        for client_sock, agency_id in pendientes:
            try:
                client_sock.close()
            except Exception as e:
                logging.error(f"action: close_pending_connection | result: fail | agency: {agency_id} | error: {e}")

    def remove_from_pending(self, client_sock):
        """
        Remueve una conexión específica de la lista de pendientes.
        """
        self.pending_winners_queries = [
            (sock, agency_id) for sock, agency_id in self.pending_winners_queries 
            if sock != client_sock
        ]

    def is_connection_pending(self, client_sock):
        """
        Verifica si una conexión está en la lista de pendientes.
        """
        return any(sock == client_sock for sock, _ in self.pending_winners_queries)