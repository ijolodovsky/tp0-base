import struct
import logging
from common.utils import Bet

def read_bets(sock) -> list[Bet]:
    """
    Lee las apuestas enviadas por el cliente usando longitud-prefijada (2 bytes) y separador '|'.
    """
    try:
        header = _read_n_bytes(sock, 2)
    except ConnectionError as e:
        logging.info("Client closed connection before sending header")
        return []

    if not header:
        raise ConnectionError("No header received")

    message_length = struct.unpack('>H', header)[0]
    logging.debug(f"Reading message of length: {message_length}")

    try:
        data = _read_n_bytes(sock, message_length)
    except ConnectionError as e:
        # Si se recibió algo, procesar lo que se tenga
        if e.args and len(e.args) > 1 and isinstance(e.args[1], bytes) and e.args[1]:
            data = e.args[1]
            logging.warning(f"Partial data received: {len(data)}/{message_length} bytes")
        else:
            raise

        text = data.decode('utf-8')
        bets_raw = text.split('\n')
        bets = []

        for line in bets_raw:
            if not line.strip():
                continue
            fields = line.split('|')
            if len(fields) != 6:
                logging.error(f"Invalid bet line: {line}")
                continue
            try:
                bet = Bet(
                    agency=fields[0],
                    first_name=fields[1],
                    last_name=fields[2],
                    document=fields[3],
                    birthdate=fields[4],
                    number=int(fields[5])
                )
                bets.append(bet)
            except Exception as ex:
                logging.error(f"Failed to parse bet: {line} | error: {ex}")
                continue

        return bets

def _read_n_bytes(sock, n: int) -> bytes:
    """
    Lee exactamente n bytes del socket, maneja short reads.
    """
    buf = b""
    while len(buf) < n:
        chunk = sock.recv(n - len(buf))
        if not chunk:
            if buf:
                raise ConnectionError("Socket closed before reading all bytes", buf)
            raise ConnectionError("Socket closed before reading all bytes")
        buf += chunk
    return buf


def send_ack(sock, bets: list[Bet]):
    """
    Envía un ACK de 4 bytes big-endian con el número de la ultima apuesta como confirmacion.
    """
    if not bets:
        return

    ack_number = bets[-1].number
    ack = struct.pack('>I', ack_number)
    _send_all(sock, ack)

def _send_all(sock, data: bytes):
    """
    Envía todos los datos, maneja short writes.
    """
    total_sent = 0
    while total_sent < len(data):
        sent = sock.send(data[total_sent:])
        if sent == 0:
            raise ConnectionError("Socket connection broken")
        total_sent += sent

def send_error_ack(sock):
    """
    Envía un ACK de error (-1) al cliente.
    """
    ack = struct.pack('>i', -1)  # entero con signo de 4 bytes
    _send_all(sock, ack)