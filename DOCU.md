## EJ3
Implementé el script `validar-echo-server.sh` en la raíz del proyecto para verificar automáticamente el funcionamiento del servidor echo. El script utiliza netcat dentro de un contenedor BusyBox conectado a la red interna de Docker, envía un mensaje al servidor y espera recibir exactamente el mismo mensaje como respuesta. Si la validación es exitosa, imprime `action: test_echo_server | result: success`, y si falla, imprime `action: test_echo_server | result: fail`. De este modo, no es necesario instalar netcat en la máquina host ni exponer puertos del servidor.

## EJ2 - Configuración Externa con Volúmenes Docker

### Implementación
Modifiqué el cliente y el servidor para que lean su configuración desde archivos externos (`config.yaml` para el cliente y `config.ini` para el servidor), que se montan como volúmenes en los contenedores Docker. Así, cualquier cambio en la configuración se aplica sin necesidad de reconstruir las imágenes, solo actualizando los archivos en el host.

## EJ1 - Generación Dinámica de Docker Compose

### Implementación
Implementé el script `generar-compose.sh` en la raíz del proyecto que genera dinámicamente un archivo Docker Compose con la cantidad de clientes especificada.

### Funcionalidad
- **Uso:** `./generar-compose.sh <archivo_salida> <cantidad_clientes>`
- **Ejemplo:** `./generar-compose.sh docker-compose-dev.yaml 5`
- **Validación:** Verifica que se proporcionen exactamente 2 parámetros
- **Nomenclatura:** Genera clientes con nombres `client1`, `client2`, `client3`, etc.

### Estructura del archivo generado
1. **Cabecera:** Define el nombre del proyecto y el servicio servidor
2. **Clientes:** Genera dinámicamente servicios cliente con:
   - Variables de entorno específicas (`CLI_ID`)
   - Dependencia del servidor
   - Configuración de red compartida
3. **Red:** Define la red `testing_net` con subred específica

### Configuración
- **Servidor:** Puerto 12345, nivel de logging DEBUG
- **Clientes:** ID únicos, logging DEBUG, conectados a la misma red
- **Red:** Subred 172.25.125.0/24 para comunicación interna

### Ejecución
El script valida parámetros, genera el archivo completo y confirma la creación exitosa.l.