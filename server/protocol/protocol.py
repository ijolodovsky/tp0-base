import struct
from common.utils import Bet

def read_bets(sock) -> list[Bet]:
    """
    Lee las apuestas enviadas por el cliente usando longitud-prefijada (2 bytes) y separador '|'.
    """
    header = _read_n_bytes(sock, 2)
    if not header:
        raise ConnectionError("No header received")

    message_length = struct.unpack('>H', header)[0]
    data = _read_n_bytes(sock, message_length)
    text = data.decode('utf-8')

    # divido el payload en las distintas apuestas
    bets_raw = text.split('\n')
    bets = []

    for line in bets_raw:
        fields = line.split('|')
        if len(fields) != 6:
            raise ValueError(f"Invalid bet received, expected 6 fields but got {len(fields)}")
        bet = Bet(
            agency=fields[0],
            first_name=fields[1],
            last_name=fields[2],
            document=int(fields[3]),
            birthdate=fields[4],
            number=int(fields[5])
        )
        bets.append(bet)

    return bets

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