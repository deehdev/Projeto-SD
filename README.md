📦 Parte 5 — Consistência e Replicação
🎯 Objetivo

O broker distribui mensagens entre os servidores usando round-robin, o que significa que:

cada servidor recebe apenas uma parte das mensagens,

se um servidor cair, sua parte do histórico é perdida,

quando um cliente pede /history, ele recebe apenas o que aquele servidor viu.

📌 Objetivo da Parte 5:
Garantir que todos os servidores mantenham uma cópia completa do histórico de mensagens, independentemente da carga distribuída pelo broker.
🧠 Método Escolhido: Replicação Epidêmica (Gossip)

Para esta parte foi implementada uma variação simples de Epidemic Replication (Gossip), um método eficiente onde cada servidor espalha mensagens para os demais até que todo o sistema esteja sincronizado.

✔ Por que esse método?

Leve e fácil de implementar

Não requer coordenação forte

Funciona bem com topologia PUB/SUB do projeto

Convergência rápida

Tolerante a falhas
🔧 Funcionamento da Replicação
1️⃣ Cliente envia uma mensagem a um servidor (via REQ/REP):

O servidor cria:

message_id (UUID v4)

origin (nome do servidor)

timestamp (RFC3339)

Adiciona ao seu messageStore

Replica a mensagem via PUB para os demais servidores

2️⃣ Todos os servidores têm um SUB ouvindo o tópico:
replication


Quando recebem uma mensagem replicada:

Deserializam com MessagePack

Salvam no messageStore

Não re-replicam (para evitar loops)

3️⃣ Quando qualquer servidor recebe /history:

Retorna todo seu histórico local

Que é idêntico em todos servidores
📡 Formato das mensagens replicadas
{
  "message_id": "uuid-v4",
  "origin": "server_a",
  "timestamp": "2025-11-16T20:00:00Z",
  "text": "Olá mundo!"
}

🔄 Fluxo Geral
sequenceDiagram
    participant Client
    participant Server_A
    participant Server_B
    participant Server_C
    participant Proxy

    Client->>Server_A: send("Olá")
    Server_A->>Server_A: armazena mensagem
    Server_A->>Proxy: PUB replicate(msg)
    Proxy->>Server_B: deliver replicate(msg)
    Proxy->>Server_C: deliver replicate(msg)
    Server_B->>Server_B: armazena mensagem
    Server_C->>Server_C: armazena mensagem
    Client->>Server_C: history
    🧪 Como Testar
1️⃣ Suba todo o sistema
docker compose up --build

2️⃣ Envie mensagem via cliente
python client.py send "Olá parte 5!"

3️⃣ Peça histórico a qualquer servidor
python client.py history

✔ Todos devem retornar o mesmo histórico.
🏁 Conclusão

A Parte 5 foi implementada usando replicação epidêmica (gossip), garantindo que:

Nenhuma mensagem é perdida

Todos os servidores convergem para o mesmo histórico

O sistema continua funcionando mesmo com falhas

Não precisa de coordenação, clock ou eleição

Funciona perfeitamente com ZeroMQ e PUB/SUB
    Server_C->>Client: histórico completo
