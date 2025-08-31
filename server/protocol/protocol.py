import struct
from common.utils import Bet
from typing import List
import logging

def read_bet(sock) -> Bet:
    """
    Lee una apuesta enviada por el cliente usando longitud-prefijada (2 bytes) y separador '|'.
    """
    bets = read_bet_batch(sock)
    if len(bets) != 1:
        raise ValueError(f"Expected single bet but got {len(bets)}")
    return bets[0]

def read_bet_batch(sock) -> List[Bet]:
    """
    Lee un batch de apuestas enviadas por el cliente usando longitud-prefijada (2 bytes).
    Cada apuesta está separada por '|' y los bets están separados por '\n'.
    """
    header = _read_n_bytes(sock, 2)
    if not header:
        raise ConnectionError("No header received")

    message_length = struct.unpack('>H', header)[0]
    data = _read_n_bytes(sock, message_length)
    text = data.decode('utf-8')

    bets = []
    bet_lines = text.split('\n')
    
    for bet_line in bet_lines:
        if not bet_line.strip():  # Skip empty lines
            continue
            
        fields = bet_line.split('|')
        if len(fields) != 6:
            raise ValueError(f"Invalid bet received, expected 6 fields but got {len(fields)}: {bet_line}")

        bet = Bet(
            agency=fields[0],
            first_name=fields[1],
            last_name=fields[2],
            document=fields[3],
            birthdate=fields[4],
            number=fields[5]
        )
        bets.append(bet)
    
    return bets

def _read_n_bytes(sock, n: int) -> bytes:
    """
    Lee exactamente n bytes del socket, maneja short reads.
    """
    buf = b""
    read_count = 0
    while len(buf) < n:
        chunk = sock.recv(n - len(buf))
        read_count += 1
        logging.debug(f"Read attempt {read_count}: expected {n - len(buf)} bytes, got {len(chunk)} bytes")
        if not chunk:
            if len(buf) == 0:
                raise ConnectionError("No data received - socket closed immediately")
            else:
                raise ConnectionError(f"Socket closed before reading all bytes - expected {n}, got {len(buf)}")
        buf += chunk
    
    logging.debug(f"Successfully read {len(buf)} bytes in {read_count} attempts")
    return buf

def send_ack(sock, bet: Bet):
    """
    Envía un ACK de 4 bytes big-endian con el número de la apuesta.
    """
    ack = struct.pack('>I', bet.number)
    _send_all(sock, ack)

def send_batch_ack(sock, success: bool):
    """
    Envía un ACK para un batch de apuestas.
    success: True si todas las apuestas fueron procesadas correctamente, False en caso contrario.
    """
    ack = struct.pack('B', 1 if success else 0)
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