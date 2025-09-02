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
                client_sock, addr = self.__accept_new_connection()
                if client_sock:
                    self.__handle_client_connection(client_sock)
            except OSError:
                # Socket was closed during shutdown
                if self.running:
                    logging.error("action: accept_connections | result: fail | error: Socket error")
                break

    def handle_sigterm(self, signum, frame):
        """
        Handle SIGTERM signal for graceful shutdown
        """
        logging.info('action: shutdown | result: in_progress')
        
        # Stop accepting new connections first
        self.running = False
        
        # Close server socket to unblock accept() call
        try:
            self._server_socket.close()
            logging.info('action: server_socket_closed | result: success')
        except Exception as e:
            logging.error(f'action: server_socket_closed | result: fail | error: {e}')
        
        logging.info('action: shutdown | result: success')

    def __handle_client_connection(self, client_sock):
        """
        Lee múltiples batches de apuestas
        """
        client_connected = True

        try:
            while client_connected:
                bets = []
                batch_fail = False
                try:
                    bets = read_bet_batch(client_sock)
                except ConnectionError:
                    logging.info("Cliente desconectado")
                    client_connected = False
                except Exception as e:
                    batch_fail = True
                    # Si hubo error, igual loguear intento de batch (fail, cantidad 0)
                    logging.info(f"action: apuesta_recibida | result: fail | cantidad: 0")
                    send_batch_ack(client_sock, 0)  # 0 indica que ninguna apuesta fue procesada
                    client_connected = False

                # Si no hay bets o la conexión se cerró, terminamos el loop
                if not bets and not batch_fail:
                    client_connected = False

                # Procesar batch si existe
                if bets:
                    last_processed_bet_number = 0
                    try:
                        store_bets(bets)
                        for bet in bets:
                            logging.info(
                                f"action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}"
                            )
                            last_processed_bet_number = bet.number
                        
                        logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")
                    except Exception as e:
                        logging.error(f"action: store_bets | result: fail | error: {e}")
                        logging.info(f"action: apuesta_recibida | result: fail | cantidad: {len(bets)}")

                    send_batch_ack(client_sock, last_processed_bet_number)

        finally:
            try:
                client_sock.close()
                logging.info('action: client_socket_closed | result: success')
            except Exception as e:
                logging.error(f"action: client_socket_close | result: fail | error: {e}")

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