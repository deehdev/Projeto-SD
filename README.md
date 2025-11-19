<div align="center">

# 💬 **Sistema Distribuído de Troca de Mensagens**
### **ZeroMQ • MessagePack • Lamport Clock • Eleição Bully • Berkeley Sync • Docker**

<img height="180" src="https://i.imgur.com/CHy8Cbu.png"/>

<br><br>

📡 Mensagens privadas — 📨 Canais públicos — 🤖 Bots automáticos — 🔁 Replicação — ⏱ Sincronização  
**Projeto completo para a disciplina BCSL502 – Sistemas Distribuídos**

---

</div>

# 🌐 **Visão Geral**

Este projeto implementa um sistema distribuído robusto inspirado em IRC/BBS, permitindo:

- Comunicação em tempo real  
- Replicação ativa entre servidores  
- Balanceamento via broker  
- Sincronização de relógios  
- Persistência em disco  
- Tolerância a falhas com eleição automática  

A arquitetura é composta por **9 containers**, todos conectados através do Docker Compose:

- 🖥 3 servidores distribuídos  
- 📡 1 proxy PUB/SUB  
- 🔄 1 broker REQ/REP  
- 📍 Servidor de referência  
- 🤖 2 bots automáticos  
- 👤 1 cliente interativo  

---

# 🧱 **Arquitetura Completa**

