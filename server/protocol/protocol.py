from common.utils import Bet
from typing import List
import logging

def parse_bet_batch_content(content: str) -> List[Bet]:
    """
    Parsea el contenido de un batch de apuestas que ya fue leído.
    """
    bets = []
    bet_lines = content.split('\n')
    
    for bet_line in bet_lines:
        if not bet_line.strip():
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
    
    return buf


def send_ack(sock, success: bool):
    """
    Envía un ACK para un batch de apuestas.
    success: True si todas las apuestas fueron procesadas correctamente, False en caso contrario.
    """
    # ACK manual: 1 byte, 1 si éxito, 0 si no
    ack = bytes([1 if success else 0])
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

def read_message(sock) -> tuple[str, str]:
    """
    Lee un mensaje genérico y determina su tipo.
    Retorna (tipo_mensaje, contenido)
    Tipos: 'BATCH_APUESTAS', 'FIN_APUESTAS', 'CONSULTA_GANADORES'
    """
    header = _read_n_bytes(sock, 2)
    if not header:
        raise ConnectionError("No header received")

    # Decodificar header manualmente (2 bytes big-endian)
    message_length = (header[0] << 8) | header[1]
    data = _read_n_bytes(sock, message_length)
    text = data.decode('utf-8')

    if text.startswith('FIN_APUESTAS|'):
        agency_id = text.split('|')[1]
        return ('FIN_APUESTAS', agency_id)
    elif text.startswith('CONSULTA_GANADORES|'):
        agency_id = text.split('|')[1]
        return ('CONSULTA_GANADORES', agency_id)
    else:
        # Es un batch de apuestas (formato actual)
        return ('BATCH_APUESTAS', text)


def send_winners_list(sock, winners_dni: List[str], sorteoRealizado: bool):
    """
    Envía la lista de DNI ganadores.
    Formato: "DNI1|DNI2|DNI3|..."
    """
    if not sorteoRealizado:
        payload = "ERROR_NO_SORTEO"
    elif not winners_dni:
        payload = ""
    else:
        # Construir el string
        payload = ""
        for i, dni in enumerate(winners_dni):
            payload += dni
            if i < len(winners_dni) - 1:
                payload += "|"

    data = payload.encode('utf-8')
    length = len(data)

    # Header de 2 bytes big-endian manual
    header = bytes([(length >> 8) & 0xFF, length & 0xFF])
    _send_all(sock, header)
    _send_all(sock, data)