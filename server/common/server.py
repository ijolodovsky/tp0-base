import signal
import socket
import logging

from common.utils import store_bets
from protocol.protocol import read_bet, send_ack, read_bet_batch, send_batch_ack

class Server:
    def __init__(self, port, listen_backlog):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self.running = True
        self.client_connections = []

        signal.signal(signal.SIGTERM, self.handle_sigterm)

    def run(self):
        while self.running:
            try:
                client_sock, addr = self._server_socket.accept()
                logging.info(f"action: accept_connections | result: success | ip: {addr[0]}")
                self.__handle_client_connection(client_sock)
            except OSError:
                break

    def handle_sigterm(self, signum, frame):
        logging.info(f'action: shutdown | result: in_progress')
        self.running = False
        self._server_socket.close()
        logging.info(f'action: shutdown | result: success')

    def __handle_client_connection(self, client_sock):
        """
        Read message from a specific client socket and closes the socket

        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        client_addr = client_sock.getpeername()
        logging.debug(f"action: handle_connection_start | client: {client_addr}")
        
        try:
            # Intentar leer como batch primero
            logging.debug(f"action: reading_batch | client: {client_addr}")
            bets = read_bet_batch(client_sock)
            logging.debug(f"action: batch_read_success | client: {client_addr} | bets_count: {len(bets)}")
            
            # Procesar todas las apuestas del batch
            all_success = True
            try:
                store_bets(bets)
                for bet in bets:
                    logging.info(f"action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}")
            except Exception as e:
                all_success = False
                logging.error(f"action: store_bets | result: fail | error: {e}")
            
            # Logging según especificaciones
            if all_success:
                logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")
            else:
                logging.info(f"action: apuesta_recibida | result: fail | cantidad: {len(bets)}")
            
            # Enviar respuesta según el resultado
            logging.debug(f"action: sending_batch_ack | client: {client_addr} | success: {all_success}")
            send_batch_ack(client_sock, all_success)
            logging.debug(f"action: batch_ack_sent | client: {client_addr}")
            
        except Exception as e:
            logging.error(f"action: receive_message | result: fail | client: {client_addr} | error: {e}")
        finally:
            logging.debug(f"action: closing_connection | client: {client_addr}")
            client_sock.close()
            logging.debug(f"action: connection_closed | client: {client_addr}")