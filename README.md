# 💬 Sistema de Mensagens Distribuído (ZMQ, MessagePack, Clocks)

Este projeto implementa um sistema de chat distribuído robusto, utilizando a arquitetura de microsserviços e os padrões de comunicação assíncrona **ZeroMQ (ZMQ)**. O sistema foca em resiliência, ordenação causal de eventos (Relógio Lógico) e consistência de dados (Replicação Ativa Síncrona).

***

## ⚙️ 1. Arquitetura e Componentes ZMQ

A arquitetura utiliza o princípio da segregação de responsabilidades com múltiplos componentes interconectados, sendo o **MessagePack** o protocolo de serialização principal.

| Componente | Linguagem | Função Principal | Padrões ZMQ | Exemplo de Endereço |
| :--- | :--- | :--- | :--- | :--- |
| **Servidor** (Réplicas) | Go | Lógica de Negócio, Persistência, **Replicação Ativa Síncrona**. | REP, PUB, REQ (Entre Pares) | REP: `tcp://*:5555` |
| **Broker** | - | Distribuição de carga de requisições **REQ/REP** de clientes (Round-Robin). | REQ-REP | `tcp://broker:5555` |
| **Proxy** | Go | Encaminhamento de mensagens **PUB/SUB** (canais e privadas) para os clientes. | XPUB-XSUB | XPUB: `tcp://*:5558` |
| **Referência (REF)** | Go | Serviço de descoberta, atribuição de **Rank** e *Heartbeat* para a sincronização de servidores. | REP | `tcp://ref:6000` |
| **Client** | Node.js/Python | Clientes que interagem (Login, Publicar, Receber Mensagens). | REQ/SUB | SUB → `tcp://proxy:5560` |
| **Bot** | Python | Geração de tráfego, conversas públicas, privadas, criação de canais. | REQ/SUB | REQ → Broker: tcp://broker:5555, SUB → Proxy: tcp://proxy:5556 |

***

## 📦 2. Protocolo de Comunicação e Serialização

Todas as mensagens trocadas entre Clientes/Bots e Servidores são serializadas usando **MessagePack**, garantindo uma comunicação binária eficiente.

### Envelope Padrão de Mensagem

Toda comunicação utiliza um `Envelope` que transporta os metadados de sincronização:

| Campo | Tipo | Descrição |
| :--- | :--- | :--- |
| `service` | string | O serviço solicitado (ex: `login`, `publish`). |
| `data` | map | Carga útil (payload) da mensagem. |
| `timestamp` | string | Timestamp físico (ISO 8601) da criação da mensagem. |
| `clock` | int | Valor do **Relógio Lógico de Lamport** do processo remetente. |

**Exemplo de Payload de Requisição (Python Bot):**
```python
# Requisição de Login (REQ -> REP)
env = {
    "service": "login",
    "data": {"user": "nome_aleatorio"},
    "clock": logical_clock,
    #...
}

## 📡 Tabela de Serviços do Protocolo

| Serviço | Padrão | Descrição | Exemplo de Uso |
|--------|--------|-----------|----------------|
| `login`, `channels`, `channel`, `subscribe`, `unsubscribe`, `heartbeat` | REQ-REP | Gerenciamento de estado e controle de sessão. | `send_req(req, "login", {"user": username})` |
| `publish` | REQ → PUB | Cliente envia REQ, Servidor responde e publica a mensagem no tópico do Canal. | `send_req(req, "publish", {"channel": "Geral", "msg": "Olá"})` |
| `message` | REQ → PUB | Cliente envia REQ, Servidor responde e publica no tópico do Usuário de Destino. | `send_req(req, "message", {"dst": "Pedro", "msg": "Privado"})` |
# Sistema de Mensagens Distribuído (ZeroMQ • MessagePack • Lamport)

Este projeto implementa um sistema de chat distribuído com tolerância a falhas, ordenação causal e replicação ativa síncrona.

---

## 1. Arquitetura dos Componentes

| Componente | Linguagem | Função | Padrões ZMQ | Endereço |
|------------|-----------|--------|-------------|----------|
| Servidor (N réplicas) | Go | Lógica, Persistência, Replicação | REP, REQ, PUB | tcp://*:5555 |
| Broker | - | Balanceamento REQ/REP | REQ-REP | tcp://broker:5555 |
| Proxy | Go | Encaminhamento PUB/SUB | XSUB- XPUB | tcp://*:5558 |
| REF | Go | Descoberta, Rank, Heartbeat | REP | tcp://ref:6000 |
| Clientes/Bots | Node/Python | Login, Publish, Receber Mensagens | REQ/SUB | tcp://proxy:5560 |
---

# ⚙️ 1. Arquitetura e Componentes

| Componente | Linguagem | Função | Padrões ZMQ | Endereço |
|-----------|-----------|--------|-------------|----------|
| Servidor (réplicas) | Go | Lógica, persistência, replicação | REP, REQ, PUB | tcp://*:5555 |
| Broker | Go | Balanceamento REQ/REP | REQ-REP | tcp://broker:5555 |
| Proxy | Go | Encaminhamento PUB/SUB | XSUB ↔ XPUB | tcp://*:5558 |
| REF | Go | Heartbeat, Rank, descoberta | REP | tcp://ref:6000 |
| Client | Node/Python | Login, publish, subscrições | REQ/SUB | tcp://proxy:5560 |
| Bot | Python | Tráfego automático | REQ/SUB | broker + proxy |

---

# 📦 2. Protocolo de Comunicação (Envelope MessagePack)

| Campo | Tipo | Descrição |
|--------|------|------------|
| service | string | Comando (`login`, `message`, etc.) |
| data | map | Payload |
| timestamp | string | Tempo físico (ISO) |
| clock | int | Clock lógico Lamport |

Exemplo (Python):

```python
env = {
    "service": "login",
    "data": {"user": "nome"},
    "clock": logical_clock
}

## 📌 Tecnologias Utilizadas

ZeroMQ (REQ/REP, XSUB/XPUB)

MessagePack

Go (servidores, broker, proxy, ref)

Node.js (cliente interativo)

Python (bot automático)

Docker & Docker Compose

Persistência em disco (JSON / arquivos)

Clock de Lamport

Sincronização Berkeley

Replicação completa entre servidores

## 🧩 Arquitetura Geral

┌──────────┐      REQ/REP       ┌──────────┐
│ Clientes │ <-----------------> │ Broker   │
│ + Bots   │                     └────┬─────┘
└─────┬────┘                          │
      │                          XSUB │ XPUB
      │                               ▼
      │                         ┌──────────┐
      │                         │  Proxy   │
      │                         └──────────┘
      │                               ▲
      ▼                               │
┌────────────┐   Sync + Replicação    │
│ Servidor A │ <-----------------------┐
├────────────┤ <---------------------->│
│ Servidor B │ <-----------------------┘
├────────────┤
│ Servidor C │
└────────────┘
       ▲
       │ Rank + Heartbeat
       ▼
   ┌────────┐
   │  REF   │
   └────────┘


## 🧱 Componentes do Sistema ##
Broker

Balanceador REQ/REP que recebe requisições dos clientes e distribui entre os servidores via round-robin.

Proxy

Encaminha todas as mensagens PUB/SUB entre servidores e clientes.

REF

Gerencia:

lista de servidores vivos

heartbeat

rank

eleição

Servidores (server_sync)

Responsáveis por:

registrar usuários

criar canais

enviar mensagens públicas

enviar mensagens privadas

sincronização via Lamport

sincronização via Berkeley

replicação completa do estado

persistência em disco

Cliente Interativo

Permite:

login

listar usuários

listar canais

criar canais

assinar canais

publicar mensagens

enviar mensagens privadas

verificar clock lógico

consultar histórico local

Bots Automáticos

Simulam usuários enviando mensagens continuamente.

## 📦 Persistência ##
Servidor
server_sync/data/
  ├── users.json
  ├── channels.json
  ├── subscriptions.json
  └── logs.json

Cliente
client/data/<username>.log

## 🔁 Replicação entre Servidores ##

O sistema utiliza Full Replication, enviando para todos os servidores um update envelope sempre que ocorre:

login

criação de canal

inscrição

mensagem pública

mensagem privada

Cada servidor recebe, aplica e persiste a atualização.

## 👑 Eleição e Sincronização  ##
Eleição

Baseada em:

rank inicial do REF

clock lógico (Lamport)

Funções do Coordenador

inicia sincronização Berkeley

ajusta offsets de tempo

coordena servidores

Clock de Lamport

Usado para ordering lógico dos eventos.

## 📡 Comunicação ##
Entre Cliente ↔ Broker

REQ/REP MessagePack

Entre Servidor ↔ Proxy

PUB/SUB texto

Entre Servidor ↔ Servidor

REQ/REP MessagePack
Para replicação e sincronização.

## 🐳 Como Executar ##
Build + subir todo o sistema
docker compose up --build

Derrubar um servidor para testar eleição
docker stop server_a

Ver coordenador sendo eleito automaticamente

Logs dos servidores irão mostrar:

[ELECTION] New coordinator: server_b

## 🎮 Comandos do Cliente Interativo ##
Comando	Função
login <nome>	cria/entra com usuário
users	lista usuários online
channels	lista canais
channel <nome>	cria canal
subscribe <canal>	inscreve no canal
publish <canal> <msg>	mensagem pública
message <user> <msg>	mensagem privada
clock	mostra clock lógico
history	mostra log local
exit	fecha cliente

## 🤖 Bots Automáticos ##

Bots Python:

escolhem nome automaticamente

fazem login

se inscrevem em canais aleatórios

enviam mensagens contínuas

enviam mensagens privadas aleatórias

mantêm heartbeat

Perfeitos para testar carga e replicação.

## 📁 Estrutura do Repositório ##
/
├── broker/
│   ├── broker.go
│   └── Dockerfile
│
├── proxy/
│   ├── proxy.go
│   └── Dockerfile
│
├── ref/
│   ├── ref.go
│   └── Dockerfile
│
├── server_sync/
│   ├── server_sync.go
│   ├── data/
│   └── Dockerfile
│
├── client/
│   ├── client.js
│   ├── data/
│   └── Dockerfile
│
├── bot/
│   ├── bot.py
│   └── Dockerfile
│
└── docker-compose.yml

## 🧪 Testes Recomendados ##

criar vários usuários simultaneamente

criar vários canais

enviar mensagens públicas e privadas

derrubar servidores e observar a réplica

levantar servidor morto e verificar sincronização

medir ordering lógico via Lamport

testar bots em paralelo

## 🏁 Conclusão ##

Este projeto demonstra:

comunicação distribuída real

tolerância a falhas

ordering lógico

replicação consistente

sincronização de tempo

arquitetura escalável com ZeroMQ

Ele integra todos os conceitos fundamentais da disciplina de Sistemas Distribuídos e serve como um framework pronto para extensões futuras.
