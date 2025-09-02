from common.utils import Bet

def read_bet(sock) -> Bet:
    """
    Lee una apuesta enviada por el cliente usando longitud-prefijada (2 bytes) y separador '|'.
    """
    header = _read_n_bytes(sock, 2)
    if not header:
        raise ConnectionError("No header received")
    message_length = (header[0] << 8) | header[1]
    data = _read_n_bytes(sock, message_length)
    text = data.decode('utf-8')

    fields = text.split('|')
    if len(fields) != 6:
        raise ValueError(f"Invalid bet received, expected 5 fields but got {len(fields)}")

    return Bet(
        agency=fields[0],  # Default agency, or you can get it from client connection
        first_name=fields[1],
        last_name=fields[2],
        document=int(fields[3]),
        birthdate=fields[4],
        number=int(fields[5])
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
    n = bet.number
    ack = bytes([
        (n >> 24) & 0xFF,
        (n >> 16) & 0xFF,
        (n >> 8) & 0xFF,
        n & 0xFF
    ])
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