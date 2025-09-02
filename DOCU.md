## EJ8: Concurrencia en el Servidor

### Implementación de Concurrencia
El servidor fue modificado para soportar múltiples clientes concurrentes usando **ThreadPoolExecutor** de Python con un máximo de 10 workers simultáneos.

### Consideraciones del GIL (Global Interpreter Lock)
Aunque Python tiene limitaciones de multithreading debido al GIL, la implementación es apropiada porque:
- El trabajo principal es **I/O-bound** (operaciones de red y archivo)
- Los threads pasan la mayor parte del tiempo esperando I/O, liberando el GIL
- Las operaciones de CPU son mínimas (parsing y serialización)

### Mecanismos de Sincronización
Se implementaron dos niveles de sincronización:

1. **Lock principal (`self.lock`)**: Protege variables compartidas como:
   - `finishedAgencies`: Contador de agencias que terminaron
   - `pending_winners_queries`: Lista de clientes esperando resultados
   - `sorteoRealizado`: Flag del estado del sorteo

2. **Lock de archivo (`self.file_lock`)**: Protege específicamente las operaciones de archivo:
   - `store_bets()`: Escritura de apuestas al CSV
   - `load_bets()`: Lectura de apuestas desde CSV
   - `has_won()`: Verificación de ganadores

### Flujo de Concurrencia
1. **Servidor principal**: Loop principal acepta conexiones
2. **Thread pool**: Cada conexión se delega a un worker thread
3. **Conexión persistente**: Un cliente mantiene una sola conexión durante todo su ciclo:
   - Envío de múltiples batches de apuestas
   - Notificación de finalización
   - Consulta de ganadores (puede quedar en espera)
4. **Sincronización del sorteo**: El servidor espera que todas las agencias terminen antes de procesar consultas de ganadores