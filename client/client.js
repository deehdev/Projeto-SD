// ===============================
// CLIENTE FINAL - PARTE 5 (LIMPO E CORRIGIDO)
// ===============================

const zmq = require("zeromq");
const msgpack = require("@msgpack/msgpack");
const readline = require("readline");

let logicalClock = 0;
let currentUser = null;

function incClock() {
  logicalClock++;
  return logicalClock;
}

function updateClock(recv) {
  logicalClock = Math.max(logicalClock, recv) + 1;
}

// ===============================
// CONFIG
// ===============================

const BROKER_REQ = "tcp://broker:5555";
const PROXY_SUB = "tcp://proxy:5558";

const req = new zmq.Request();
const sub = new zmq.Subscriber();

let busy = false;

// ===============================
// START
// ===============================

async function start() {
  console.log("Connected to server.");

  await req.connect(BROKER_REQ);
  console.log("[CLIENT] REQ conectado ao broker");

  await sub.connect(PROXY_SUB);
  console.log("[CLIENT] SUB conectado ao proxy (XPUB 5558)");

  startSubLoop();
  startInputLoop();
}

start();

// ===============================
// SUB LOOP
// ===============================

async function startSubLoop() {
  for await (const [topic, payload] of sub) {
    try {
      const env = msgpack.decode(payload);
      updateClock(env.clock);

      console.log(`\n[SUB:${topic}] mensagem recebida (clock=${env.clock}, local=${logicalClock})`);
      console.log(env.data);

      process.stdout.write("> ");
    } catch (err) {
      console.log("[SUB ERRO]", err);
    }
  }
}

// ===============================
// SEND (REQ/REP) com TIMEOUT REAL
// ===============================

async function send(service, data = {}) {
  if (busy) {
    console.log("Aguarde, requisição anterior ainda em andamento.");
    return;
  }
  busy = true;

  try {
    const envelope = {
      service,
      data,
      timestamp: new Date().toISOString(),
      clock: incClock()
    };

    await req.send(msgpack.encode(envelope));

    // ---- TIMEOUT correto ----
    let replyBytes;
    try {
      replyBytes = await Promise.race([
        req.receive(),
        new Promise((_, reject) => setTimeout(() => reject(new Error("TIMEOUT")), 5000))
      ]);
    } catch (e) {
      if (e.message === "TIMEOUT") {
        console.log(`❌ [TIMEOUT] O servidor não respondeu ao serviço "${service}" em 5 segundos.`);
        return;
      }
      throw e;
    }

    const [reply] = replyBytes;
    const decoded = msgpack.decode(reply);

    updateClock(decoded.clock);

    console.log(`\n[RESPOSTA:${service}] clock_local=${logicalClock}, clock_srv=${decoded.clock}`);

    // Processamento das respostas
    processReply(service, decoded, data);

    return decoded;

  } catch (err) {
    console.log("[REQ ERRO]", err);
  } finally {
    busy = false;
  }
}

// ===============================
// Tratamento unificado de respostas
// ===============================

function processReply(service, reply, data) {

  if (service === "login") {
    if (reply.data.status === "sucesso") {
      console.log(`Login realizado como "${data.user}"`);
      currentUser = data.user;
      sub.subscribe(currentUser);
      console.log(`Inscrito no tópico privado: ${currentUser}`);
    } else {
      console.log("Erro no login:", reply.data.description || "Usuário já existe");
    }
    return;
  }

  if (service === "users") {
    const list = reply.data.users || [];
    if (list.length === 0) console.log("Nenhum usuário cadastrado.");
    else list.forEach(u => console.log(" - " + u));
    return;
  }

  if (service === "channels") {
    const list = reply.data.channels || [];
    if (list.length === 0) console.log("Nenhum canal criado.");
    else list.forEach(c => console.log(" - " + c));
    return;
  }

  if (service === "channel") {
    if (reply.data.status === "sucesso")
      console.log(`Canal criado: "${data.channel}"`);
    else
      console.log("Erro ao criar canal:", reply.data.description || "Canal já existe");
    return;
  }

  if (service === "publish") {
    if (reply.data.status === "sucesso")
      console.log(`Mensagem publicada no canal "${data.channel}"`);
    else
      console.log("Erro ao publicar:", reply.data.message || "Erro desconhecido");
    return;
  }

  if (service === "message") {
    if (reply.data.status === "sucesso")
      console.log(`Mensagem enviada para "${data.dst}"`);
    else
      console.log("Erro ao enviar mensagem:", reply.data.message || "Usuário inexistente");
    return;
  }

  if (service === "subscribe") {
    console.log(`Inscrito no tópico ${data.topic}`);
    return;
  }

  if (service === "unsubscribe") {
    console.log(`Desinscrito do tópico ${data.topic}`);
    return;
  }
}

// ===============================
// INPUT
// ===============================

const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout
});

function showHelp() {
  console.log(`
Comandos disponíveis:

  login <nome>               - realizar login
  users                      - listar usuários
  channels                   - listar canais
  channel <nome>             - criar canal
  publish <canal> <msg>      - enviar mensagem no canal
  message <user> <msg>       - enviar mensagem privada
  subscribe <topic>          - entrar em um canal/tópico
  unsubscribe <topic>        - sair de um canal/tópico
  help                       - mostrar esta lista
  `);
}

function startInputLoop() {
  showHelp();

  rl.setPrompt("> ");
  rl.prompt();

  rl.on("line", async (line) => {
    const parts = line.trim().split(/\s+/);
    const command = parts[0]?.toLowerCase() || "";

    try {
      switch (command) {
        case "login":
          if (!parts[1]) console.log("Erro: faltando nome.");
          else await send("login", { user: parts[1] });
          break;

        case "users": await send("users"); break;
        case "channels": await send("channels"); break;

        case "channel":
          if (!parts[1]) console.log("Erro: faltando nome do canal.");
          else await send("channel", { channel: parts[1] });
          break;

        case "publish":
          if (!currentUser) console.log("Erro: faça login primeiro.");
          else if (parts.length < 3) console.log("Uso: publish <canal> <mensagem>");
          else await send("publish", {
            user: currentUser,
            channel: parts[1],
            message: parts.slice(2).join(" ")
          });
          break;

        case "message":
          if (!currentUser) console.log("Erro: faça login primeiro.");
          else if (parts.length < 3) console.log("Uso: message <user> <msg>");
          else await send("message", {
            src: currentUser,
            dst: parts[1],
            message: parts.slice(2).join(" ")
          });
          break;

        case "subscribe":
          if (!currentUser) console.log("Erro: faça login primeiro.");
          else if (!parts[1]) console.log("Erro: faltando tópico.");
          else {
            sub.subscribe(parts[1]);
            await send("subscribe", { user: currentUser, topic: parts[1] });
          }
          break;

        case "unsubscribe":
          if (!currentUser) console.log("Erro: faça login primeiro.");
          else if (!parts[1]) console.log("Erro: faltando tópico.");
          else {
            sub.unsubscribe(parts[1]);
            await send("unsubscribe", { user: currentUser, topic: parts[1] });
          }
          break;

        case "help": showHelp(); break;
        case "": break;
        default: console.log("Comando desconhecido. Digite: help");
      }
    } catch (err) {
      console.log("[CLIENT ERRO]", err);
    }

    rl.prompt();
  });
}
