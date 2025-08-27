## EJ1
Genero un archivo en la raiz del proyecto llamado generar-compose.sh, que hace una validacion basica de los parametros recibidos (que sean 2), luego escribe la parte inicial del docker-compose y hace un loop para generar los clientes, agregando al final la definición de red.
> Tuve que darle permiso con chmod al bash

## EJ6
Primero rearme las funciones de SendBet para que ahora sea SendBets y read_bet para que sea read_bets para enviar y recibir una o mas apuestas en un batch, donde cada apuesta va en una linea separada por '|'.
Tambien el send_ack cambia su logica para que si se proceso todo ok manda el numero de la ultima apuesta