import struct
from common.utils import Bet

def read_bet(sock) -> Bet:
    """
    Lee una apuesta enviada por el cliente usando longitud-prefijada (2 bytes) y separador '|'.
    """
    header = _read_n_bytes(sock, 2)
    if not header:
        raise ConnectionError("No header received")

    message_length = struct.unpack('>H', header)[0]
    data = _read_n_bytes(sock, message_length)
    text = data.decode('utf-8')

    fields = text.split('|')
    if len(fields) != 6:
        raise ValueError("Invalid bet received")

    return Bet(
        agency=fields[0],
        first_name=fields[1],
        last_name=fields[2],
        document=fields[3],
        birthdate=fields[4],
        number=fields[5]
    )

def _read_n_bytes(sock, n: int) -> bytes:
    """
    Lee exactamente n bytes del socket, maneja short reads.
    """
    buf = b""
    while len(buf) < n:
        chunk = sock.recv(n - len(buf))
        if not chunk:
            raise ConnectionError("Socket closed before reading all bytes")
        buf += chunk
    return buf

def send_ack(sock, bet: Bet):
    """
    Envía un ACK de 4 bytes big-endian con el número de la apuesta.
    """
    ack = struct.pack('>I', bet.number)
    sock.sendall(ack)