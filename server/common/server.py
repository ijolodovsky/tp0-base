import signal
import socket
import logging
import os

from common.utils import store_bets
from protocol.protocol import read_bets, send_ack

class Server:
    def __init__(self, port, listen_backlog, batch_max_amount=10):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self.running = True
        self.batch_max_amount = batch_max_amount
        self.client_connections = []

        signal.signal(signal.SIGTERM, self.handle_sigterm)

    def run(self):
        # Solo atiende a un cliente y luego termina
        if self.running:
            try:
                client_sock, addr = self._server_socket.accept()
                logging.info(f"action: accept_connections | result: success | ip: {addr[0]}")
                self.__handle_client_connection(client_sock)
            except OSError:
                pass
        self.running = False

    def handle_sigterm(self, signum, frame):
        logging.info(f'action: shutdown | result: in_progress')
        self.running = False
        self._server_socket.close()
        logging.info(f'action: shutdown | result: success')
        os._exit(0)

    def __handle_client_connection(self, client_sock):
        """
        Lee y procesa múltiples batches de apuestas por conexión hasta que el cliente cierre el socket.
        """
        try:
            while True:
                try:
                    bets = read_bets(client_sock, self.batch_max_amount)
                except ConnectionError:
                    logging.info("action: client_disconnected | result: success | motivo: socket cerrado por el cliente")
                    break
                except Exception as e:
                    logging.error(f"action: receive_message | result: fail | error: {e}")
                    break

                if len(bets) > self.batch_max_amount:
                    logging.error(f"action: apuesta_recibida | result: fail | cantidad: {len(bets)} | error: too_many_bets")
                    continue

                try:
                    store_bets(bets)
                    logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")
                    send_ack(client_sock, bets)
                except Exception as store_error:
                    logging.error(f"action: apuesta_recibida | result: fail | cantidad: {len(bets)} | error: {store_error}")
        finally:
            client_sock.close()