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
            client_sock.close()
            logging.info("Conexión con cliente cerrada")