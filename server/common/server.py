import signal
import socket
import logging

from common.utils import store_bets, load_bets, has_won
from protocol.protocol import read_message, send_batch_ack, send_simple_ack, send_winners_list, parse_bet_batch_content

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

        signal.signal(signal.SIGTERM, self.handle_sigterm)
        
        # Log de configuración
        logging.info(f"action: config | result: success | expected_agencies: {expected_agencies}")

    def run(self):
        while self.running:
            try:
                client_sock, addr = self.__accept_new_connection()
                if client_sock:
                    self.__handle_client_connection(client_sock)
            except OSError:
                if self.running:
                    logging.error("action: accept_connections | result: fail | error: Socket error")
                break

    def handle_sigterm(self, signum, frame):
        """
        Handle SIGTERM signal for graceful shutdown
        """
        logging.info('action: shutdown | result: in_progress')
        
        self.running = False
        
        # Cerrar todas las conexiones pendientes de consultas de ganadores
        self.close_pending_connections()
        
        try:
            self._server_socket.close()
            logging.info('action: server_socket_closed | result: success')
        except Exception as e:
            logging.error(f'action: server_socket_closed | result: fail | error: {e}')
        
        logging.info('action: shutdown | result: success')

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

                    last_processed_bet_number = 0
                    try:
                        store_bets(bets)
                        # Si llegamos aquí, todas las apuestas fueron procesadas exitosamente
                        for bet in bets:
                            last_processed_bet_number = bet.number
                        
                        logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")
                    except Exception as e:
                        logging.error(f"action: apuesta_recibida | result: fail | error: {e}")
                        logging.info(f"action: apuesta_recibida | result: fail | cantidad: {len(bets)}")
                    
                    send_batch_ack(client_sock, last_processed_bet_number)
                
                elif msg_type == 'FIN_APUESTAS':
                    agency_id = content
                    self.finishedAgencies += 1
                    # mira si ya terminaron todas las agencias esperadas
                    if self.finishedAgencies == self.expected_agencies and not self.sorteoRealizado:
                        self.sorteoRealizado = True
                        logging.info('action: sorteo | result: success')
                        
                        # Responder a todos los clientes que estaban esperando ganadores
                        self.respond_pending_winners()
                    
                    send_simple_ack(client_sock, True)  # ACK simple para confirmación de finalización
                    
                elif msg_type == 'CONSULTA_GANADORES':
                    agency_id = content
                    
                    if not self.sorteoRealizado:
                        # Agregar cliente a la lista de espera (no cierra la conexion)
                        self.client_connections.append((client_sock, int(agency_id)))
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
                    logging.info('action: client_socket_closed | result: success')
                except Exception as e:
                    logging.error(f"action: client_socket_close | result: fail | error: {e}")

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
        for client_sock, agency_id in self.client_connections:
            try:
                winners = self.get_winners_agency(agency_id)
                send_winners_list(client_sock, winners, sorteoRealizado=True)
                logging.info(f"action: ganadores_enviados | result: success | cant_ganadores: {len(winners)} | agency: {agency_id}")
                client_sock.close()
            except Exception as e:
                logging.error(f"action: respond_pending_winners | result: fail | agency: {agency_id} | error: {e}")
                try:
                    client_sock.close()
                except:
                    pass
        
        self.client_connections.clear()

    def close_pending_connections(self):
        """
        Cierra todas las conexiones pendientes de consultas de ganadores.
        Se llama durante el shutdown para evitar conexiones abiertas.
        """
        logging.info(f"action: close_pending_connections | result: in_progress | count: {len(self.client_connections)}")
        
        for client_sock, agency_id in self.client_connections:
            try:
                client_sock.close()
                logging.info(f'action: pending_connection_closed | result: success | agency: {agency_id}')
            except Exception as e:
                logging.error(f"action: close_pending_connection | result: fail | agency: {agency_id} | error: {e}")
        
        self.client_connections.clear()
        logging.info('action: close_pending_connections | result: success')

    def remove_from_pending(self, client_sock):
        """
        Remueve una conexión específica de la lista de pendientes.
        """
        self.client_connections = [
            (sock, agency_id) for sock, agency_id in self.client_connections 
            if sock != client_sock
        ]

    def is_connection_pending(self, client_sock):
        """
        Verifica si una conexión está en la lista de pendientes.
        """
        return any(sock == client_sock for sock, _ in self.client_connections)

    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned
        """
        
        if not self.running:
            return None, None

        # Connection arrived
        logging.info('action: accept_connections | result: in_progress')
        try:
            c, addr = self._server_socket.accept()
            logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
            return c, addr
        except OSError:
            if self.running:
                logging.error('action: accept_connections | result: fail | error: Socket closed')
            return None, None