import signal
import socket
import logging
import os
import time

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

    def should_exit(self):
        """Verifica si el servidor debe terminar"""
        return not self.running

    def run(self):
        # Atiende a un cliente y procesa múltiples batches, luego termina
        logging.debug("Server run() method started")
        try:
            logging.debug("Waiting for client connection...")
            client_sock, addr = self._server_socket.accept()
            logging.info(f"action: accept_connections | result: success | ip: {addr[0]}")
            self.__handle_client_connection(client_sock)
            logging.debug("Client connection handled, server will exit")
        except OSError as e:
            logging.debug(f"OSError in accept, server will exit: {e}")
        
        # Cerrar el socket del servidor y terminar naturalmente
        if self._server_socket:
            self._server_socket.close()
            logging.debug("Server socket closed")
            # Verificar que el socket esté cerrado
            try:
                self._server_socket.getsockname()
                logging.warning("Server socket still open after close")
            except:
                logging.debug("Server socket closed successfully")
        logging.debug("Server shutting down")
        self.running = False
        logging.debug("Server run() method finished")

    def handle_sigterm(self, signum, frame):
        logging.info(f'action: shutdown | result: in_progress | motivo: SIGTERM recibido')
        self.running = False
        self._server_socket.close()
        logging.info(f'action: shutdown | result: success | motivo: SIGTERM recibido')

    def __handle_client_connection(self, client_sock):
        """
        Lee y procesa múltiples batches de apuestas por conexión hasta que el cliente cierre el socket.
        """
        logging.debug("Starting to handle client connection")
        try:
            while self.running:
                try:
                    # Leer todas las apuestas
                    bets = read_bets(client_sock)
                    logging.debug(f"Read {len(bets)} bets from client")
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
                    # Dar tiempo para que el cliente procese el ACK
                    time.sleep(0.05)  # 50ms
                except Exception as store_error:
                    # Si falla el almacenamiento, loguear como error y terminar
                    logging.error(f"action: apuesta_recibida | result: fail | cantidad: {len(bets)} | error: {store_error}")
                    break
        finally:
            client_sock.close()
            logging.debug("Client connection closed, server will exit")
            self.running = False
            logging.debug("Client connection handler finished")
            # Verificar que el socket esté cerrado
            try:
                client_sock.getpeername()
                logging.warning("Client socket still open after close")
            except:
                logging.debug("Client socket closed successfully")