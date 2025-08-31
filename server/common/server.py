import signal
import socket
import logging
import os

from common.utils import store_bets
from protocol.protocol import read_bets, send_ack

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
        # Atiende a un cliente y procesa múltiples batches, luego termina
        if self.running:
            try:
                client_sock, addr = self._server_socket.accept()
                logging.info(f"action: accept_connections | result: success | ip: {addr[0]}")
                self.__handle_client_connection(client_sock)
                logging.debug("Client connection handled, server will exit")
            except OSError:
                logging.debug("OSError in accept, server will exit")
                pass
        self.running = False
        logging.debug("Server shutting down")
        os._exit(0)

    def handle_sigterm(self, signum, frame):
        logging.info(f'action: shutdown | result: in_progress | motivo: SIGTERM recibido')
        self.running = False
        self._server_socket.close()
        logging.info(f'action: shutdown | result: success | motivo: SIGTERM recibido')
        os._exit(0)

    def __handle_client_connection(self, client_sock):
        """
        Lee y procesa múltiples batches de apuestas por conexión hasta que el cliente cierre el socket.
        """
        try:
            while True:
                try:
                    # Leer todas las apuestas
                    bets = read_bets(client_sock)
                except Exception as e:
                    # Si el error es por socket cerrado, loguear como info, no como error
                    if "Socket closed before reading all bytes" in str(e):
                        logging.info("action: client_disconnected | result: success | motivo: socket cerrado por el cliente (EOF)")
                        break
                    else:
                        logging.error(f"action: receive_message | result: fail | error: {e}")
                        break

                if len(bets) == 0:
                    logging.error(f"action: apuesta_recibida | result: fail | cantidad: 0 | error: empty_batch")
                    break

                try:
                    store_bets(bets)
                    logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")
                    send_ack(client_sock, bets)
                except Exception as store_error:
                    # Si falla el almacenamiento, loguear como error y terminar
                    logging.error(f"action: apuesta_recibida | result: fail | cantidad: {len(bets)} | error: {store_error}")
                    break
        finally:
            client_sock.close()
            logging.debug("Client connection closed, server will exit")