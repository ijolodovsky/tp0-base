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