import signal
import socket
import logging

from common.utils import store_bets
from protocol.protocol import read_bet, send_ack

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
                if self.running:
                    logging.error("action: accept_connections | result: fail | error: Socket error")
                break

    def handle_sigterm(self, signum, frame):
        """
        Handle SIGTERM signal for graceful shutdown
        """
        logging.info('action: shutdown | result: in_progress')
        
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
        Read bet from a specific client socket and closes the socket

        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        try:
            bet = read_bet(client_sock)
            store_bets([bet])
            logging.info(f"action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}")
            send_ack(client_sock, bet)
        except Exception as e:
            logging.error(f"action: receive_message | result: fail | error: {e}")
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