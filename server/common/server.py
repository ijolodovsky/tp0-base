import signal
import socket
import logging

from common.utils import store_bets, load_bets, has_won
from protocol.protocol import read_bet, read_message, send_ack, read_bet_batch, send_batch_ack, send_finish_ack, send_winners_list

class Server:
    def __init__(self, port, listen_backlog):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self.running = True
        self.client_connections = []
        self.finishedAgencies = 0
        self.sorteoRealizado = False

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
        
        try:
            msg_type, content = read_message(client_sock)

            if msg_type == 'BATCH_APUESTAS':
                # El contenido ya fue leído en read_message(), necesitamos parsearlo
                from protocol.protocol import parse_bet_batch_content
                bets = parse_bet_batch_content(content)

                # Procesar todas las apuestas del batch
                all_success = True
                try:
                    store_bets(bets)
                    for bet in bets:
                        logging.info(f"action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}")
                except Exception as e:
                    all_success = False
                    logging.error(f"action: store_bets | result: fail | error: {e}")
                
                # Logging segun resultado del batch
                if all_success:
                    logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")
                else:
                    logging.info(f"action: apuesta_recibida | result: fail | cantidad: {len(bets)}")
                
                send_batch_ack(client_sock, all_success)
            
            elif msg_type == 'FIN_APUESTAS':
                agency_id = content
                self.finishedAgencies += 1
                logging.info(f"action: agency_finished | result: success | agency: {agency_id} | total_finished: {self.finishedAgencies}")
                
                # mira si ya terminaron las 5 agencias
                if self.finishedAgencies == 5 and not self.sorteoRealizado:
                    self.sorteoRealizado = True
                    logging.info("action: sorteo | result: success")
                
                send_finish_ack(client_sock, True)
                
            elif msg_type == 'CONSULTA_GANADORES':
                agency_id = content
                
                if not self.sorteoRealizado:
                    send_winners_list(client_sock, [], sorteoRealizado=False)
                    logging.info(f"action: consulta_ganadores | result: fail | reason: sorteo_no_realizado | agency: {agency_id}")
                else:
                    # Busco los ganadores de esta agencia
                    winners = self.get_winners_agency(int(agency_id))
                    send_winners_list(client_sock, winners, sorteoRealizado=True)
                    logging.info(f"action: consulta_ganadores | result: success | cant_ganadores: {len(winners)} | agency: {agency_id}")

        except Exception as e:
            logging.error(f"action: receive_message | result: fail | error: {e}")
        finally:
            client_sock.close()

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
            logging.error(f"action: get_winners_for_agency | result: fail | agency: {agency_id} | error: {e}")
            return []