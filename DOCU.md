## EJ1
Implementé el script `generar-compose.sh` en la raíz del proyecto, que genera un archivo Docker Compose con la cantidad de clientes indicada por parámetro (ej: `./generar-compose.sh docker-compose-dev.yaml 5`).
Valida los parámetros, escribe la cabecera, agrega los servicios `client1`, `client2`, etc., y la red al final.