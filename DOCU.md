## EJ7
Modifiqué la lógica de clientes y servidor para soportar la notificación de fin de apuestas y la consulta de ganadores por agencia:

- **Cliente:** Al terminar de enviar todas las apuestas, cada cliente notifica al servidor que finalizó su carga. Luego, consulta la lista de ganadores de su agencia y loguea: `action: consulta_ganadores | result: success | cant_ganadores: ${CANT}`. El cliente espera la respuesta del servidor antes de finalizar.

La notificación y la consulta se implementan como nuevos tipos de mensaje en el protocolo, enviados por el cliente tras procesar su archivo de apuestas.

- **Servidor:** El servidor espera la notificación de las 5 agencias antes de realizar el sorteo. Una vez recibidas todas, ejecuta el sorteo (usando las funciones provistas `load_bets(...)` y `has_won(...)`), loguea el evento con: `action: sorteo | result: success`, y responde a cada consulta de ganadores solo con los DNIs correspondientes a la agencia solicitante. Antes del sorteo, las consultas quedan en espera y no se responde información parcial.

El servidor mantiene un contador de agencias finalizadas y una lista de consultas pendientes. Cuando recibe la notificación de la última agencia, realiza el sorteo y responde a todas las consultas en espera.

- **Comunicación y robustez:** El protocolo de mensajes fue extendido para incluir la notificación de fin de apuestas y la consulta de ganadores. El servidor maneja correctamente la espera de todas las agencias y la sincronización de consultas, asegurando que cada cliente reciba solo la información que le corresponde.

## EJ6
Modifiqué los clientes para que envíen apuestas en batchs (chunks), permitiendo registrar varias apuestas en una sola consulta al servidor y optimizando la transmisión y el procesamiento.

- **Cliente:** Cada cliente lee las apuestas desde un archivo CSV (`.data/agency-{N}.csv`), que se monta como volumen en el contenedor. El tamaño máximo de cada batch es configurable mediante `batch.maxAmount` en `config.yaml`, y el valor por defecto fue ajustado para que los paquetes no superen los 8kB. El cliente envía los batchs al servidor y espera la confirmación. Si el batch es aceptado, loguea: `action: apuesta_enviada | result: success | cantidad: ${CANTIDAD_DE_APUESTAS}`; si hay error, loguea el fallo y la cantidad de apuestas.

- **Servidor:** El servidor recibe los batchs, procesa todas las apuestas y responde con éxito solo si todas fueron almacenadas correctamente. Loguea: `action: apuesta_recibida | result: success | cantidad: ${CANTIDAD_DE_APUESTAS}` o, en caso de error, `action: apuesta_recibida | result: fail | cantidad: ${CANTIDAD_DE_APUESTAS}`.

- **Archivos y persistencia:** Los archivos de apuestas se inyectan y persisten por fuera de la imagen Docker usando volúmenes.

## EJ5
Modifiqué la lógica de cliente y servidor para emular el flujo de apuestas de una agencia de quiniela:

- **Cliente:** Cada cliente representa una agencia y recibe los datos de la apuesta (nombre, apellido, DNI, nacimiento, número) por variables de entorno. Estos datos se envían al servidor usando un protocolo propio basado en sockets y mensajes longitud-prefijados. Al recibir la confirmación del servidor, el cliente imprime en el log: `action: apuesta_enviada | result: success | dni: ${DNI} | numero: ${NUMERO}`.

- **Servidor:** El servidor recibe las apuestas de los clientes, las almacena usando la función provista `store_bet` y loguea cada registro con: `action: apuesta_almacenada | result: success | dni: ${DNI} | numero: ${NUMERO}`.

- **Comunicación:** Implementé un módulo de comunicación dedicado, que serializa los datos de la apuesta, define un protocolo claro (header de longitud, campos separados por `|`), y maneja correctamente short reads/writes y errores de socket.

## EJ4
Tanto el servidor como el cliente fueron modificados para implementar un shutdown graceful al recibir la señal SIGTERM:

- **Servidor:** Se utiliza `signal.signal(signal.SIGTERM, self.handle_sigterm)` para capturar la señal. Al recibirla, el servidor deja de aceptar nuevas conexiones (`self.running = False`), cierra todas las conexiones pendientes mediante `close_pending_connections()`, y finalmente cierra el socket principal (`self._server_socket.close()`).

- **Cliente:** Al recibir la señal, se ejecuta el método `Stop()`, que cierra la conexión activa (`c.conn.Close()`) y loguea el cierre y la salida del cliente. Además, en el ciclo principal (`StartClientLoop`) se verifica la señal y se interrumpe el envío de mensajes si corresponde, asegurando que no queden recursos abiertos.

## EJ3
Implementé el script `validar-echo-server.sh` en la raíz del proyecto para verificar automáticamente el funcionamiento del servidor echo. El script utiliza netcat dentro de un contenedor BusyBox conectado a la red interna de Docker, envía un mensaje al servidor y espera recibir exactamente el mismo mensaje como respuesta. Si la validación es exitosa, imprime `action: test_echo_server | result: success`, y si falla, imprime `action: test_echo_server | result: fail`. De este modo, no es necesario instalar netcat en la máquina host ni exponer puertos del servidor.

## EJ2
Modifiqué el cliente y el servidor para que lean su configuración desde archivos externos (`config.yaml` para el cliente y `config.ini` para el servidor), que se montan como volúmenes en los contenedores Docker. Así, cualquier cambio en la configuración se aplica sin necesidad de reconstruir las imágenes, solo actualizando los archivos en el host.

## EJ1
Implementé el script `generar-compose.sh` en la raíz del proyecto, que genera un archivo Docker Compose con la cantidad de clientes indicada por parámetro (ej: `./generar-compose.sh docker-compose-dev.yaml 5`).
Valida los parámetros, escribe la cabecera, agrega los servicios `client1`, `client2`, etc., y la red al final.