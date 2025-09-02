from common.utils import Bet
from typing import List
import logging

def parse_bet_batch_content(content: str) -> List[Bet]:
    """
    Convierte el texto de un batch en una lista de apuestas
    Formato: cada línea es "agencia|nombre|apellido|dni|fecha|numero"
    """
    bets = []
    bet_lines = content.split('\n')
    
    for bet_line in bet_lines:
        if not bet_line.strip():
            continue
            
        # Cada apuesta tiene 6 campos separados por |
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
    
    # Sigo leyendo hasta tener todos los bytes que necesito
    while len(buf) < n:
        chunk = sock.recv(n - len(buf))  # Pido los bytes que me faltan
        read_count += 1
        logging.debug(f"Read attempt {read_count}: expected {n - len(buf)} bytes, got {len(chunk)} bytes")
        if not chunk:
            if len(buf) == 0:
                raise ConnectionError("No data received - socket closed immediately")
            else:
                raise ConnectionError(f"Socket closed before reading all bytes - expected {n}, got {len(buf)}")
        buf += chunk
    
    return buf


def send_batch_ack(sock, last_processed_bet_number: int):
    """
    Envío confirmación de batch procesado.
    Le digo al cliente hasta qué número de apuesta procesé bien.
    """
    # Enviar 4 bytes big-endian con el número de la última apuesta procesada
    n = last_processed_bet_number
    ack = bytes([
        (n >> 24) & 0xFF,
        (n >> 16) & 0xFF,
        (n >> 8) & 0xFF,
        n & 0xFF
    ])
    _send_all(sock, ack)

def send_simple_ack(sock, success: bool):
    """
    Envío ACK simple de 1 byte para mensajes FIN_APUESTAS
    """
    # 1 byte: 1 = éxito, 0 = error
    ack = bytes([1 if success else 0])
    _send_all(sock, ack)

def _send_all(sock, data: bytes):
    """
    Envía todos los datos, maneja short writes.
    """
    total_sent = 0
    
    # Sigo enviando hasta que mande todos los bytes
    while total_sent < len(data):
        sent = sock.send(data[total_sent:])  # Envío lo que me falta
        if sent == 0:  # Si send() devuelve 0, la conexión se rompió
            raise ConnectionError("Socket connection broken")
        total_sent += sent

def read_message(sock) -> tuple[str, str]:
    """
    Lee un mensaje y determina qué tipo es.
    Protocolo: 2 bytes (longitud) + payload
    Retorna (tipo_mensaje, contenido)
    """
    # Primero leo el header de 2 bytes que me dice cuánto mide el mensaje
    header = _read_n_bytes(sock, 2)
    if not header:
        raise ConnectionError("No header received")

    # Decodifico la longitud (2 bytes big-endian)
    message_length = (header[0] << 8) | header[1]
    
    # Ahora leo el contenido del mensaje
    data = _read_n_bytes(sock, message_length)
    text = data.decode('utf-8')

    # Analizo qué tipo de mensaje es por el prefijo
    if text.startswith('FIN_APUESTAS|'):
        agency_id = text.split('|')[1]
        return ('FIN_APUESTAS', agency_id)
    elif text.startswith('CONSULTA_GANADORES|'):
        agency_id = text.split('|')[1]
        return ('CONSULTA_GANADORES', agency_id)
    else:
        # Es un batch de apuestas
        return ('BATCH_APUESTAS', text)


def send_winners_list(sock, winners_dni: List[str], sorteoRealizado: bool):
    """
    Envío la lista de ganadores al cliente.
    Formato: "DNI1|DNI2|DNI3|..." o "" si no hay ganadores
    """
    if not sorteoRealizado:
        # Esto no debería pasar nunca, pero por las dudas
        payload = "ERROR_NO_SORTEO"
    elif not winners_dni:
        # No hay ganadores en esta agencia
        payload = ""
    else:
        # Armo la lista separada por |
        payload = ""
        for i, dni in enumerate(winners_dni):
            payload += dni
            if i < len(winners_dni) - 1:
                payload += "|"

    # Convierto a bytes y armo el header
    data = payload.encode('utf-8')
    length = len(data)

    # Header de 2 bytes big-endian con la longitud
    header = bytes([(length >> 8) & 0xFF, length & 0xFF])
    _send_all(sock, header)
    _send_all(sock, data)