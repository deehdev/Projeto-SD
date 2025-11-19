// =====================================================
// CLIENTE INTERATIVO — REQ + SUB funcionando 100%
// =====================================================

const zmq = require("zeromq");
const msgpack = require("@msgpack/msgpack");
const readline = require("readline");

// -------------------------------
// Relógio lógico
// -------------------------------
let clock = 0;
function incClock() {
  clock++;
  return clock;
}
function updateClock(received) {
  received = Number(received) || 0;
  clock = Math.max(clock, received) + 1;
}

// -------------------------------
// Endereços
// -------------------------------
const REQ_ADDR = process.env.REQ_ADDR || "tcp://broker:5555";
const SUB_ADDR = process.env.SUB_ADDR || "tcp://proxy:5558";

// -------------------------------
// Sockets
// -------------------------------
const req = new zmq.Request();
const sub = new zmq.Subscriber();

let busy = false;
let currentUser = null;

// -------------------------------
// Request helper
// -------------------------------
async function send(service, data = {}) {
  if (busy) {
    console.log("⚠ O socket ainda está ocupado.");
    return;
  }

  busy = true;

  data.timestamp = new Date().toISOString();
  data.clock = incClock();

  const env = { service, data };
  await req.send(msgpack.encode(env));

  const reply = await req.receive();
  const decoded = msgpack.decode(reply[0]);

  updateClock(decoded.data.clock);

  busy = false;
  return decoded;
}

// -------------------------------
// SUB Listener (mensagens recebidas)
// -------------------------------
// -----------------------
// SUB Listener (mensagens recebidas)
// -----------------------
async function startSubListener() {
  for await (const [topicBuf, msgBuf] of sub) {
    try {
      const rawTopic = topicBuf.toString();
      const topic = rawTopic.trim().replace(/[^a-zA-Z0-9_-]/g, "");

      const env = msgpack.decode(msgBuf);
      const service = env.service;
      const data = env.data || {};

      updateClock(data.clock);

      // SALVAR O QUE O USUÁRIO ESTÁ DIGITANDO
      const typed = rl.line;

      // LIMPAR LINHA ATUAL
      readline.cursorTo(process.stdout, 0);
      readline.clearLine(process.stdout, 0);

      // ---------------------------
      // FORMATAR MENSAGEM RECEBIDA
      // ---------------------------
      if (service === "publish") {
        console.log(`💬  #${topic} | ${data.user} → ${data.message}`);
      } else if (service === "message") {
        console.log(`📩  ${data.src} → você | ${data.message}`);
      } else {
        console.log(`🔧 [${topic}]`, env);
      }

      // REIMPRIMIR PROMPT E TEXTO DIGITADO
      rl.prompt(true);
      process.stdout.write(typed);

    } catch (e) {
      console.log("Erro no SUB:", e);
    }
  }
}

// -------------------------------
// Comandos
// -------------------------------
async function cmdLogin(args) {
  const user = args[0];
  if (!user) return console.log("Uso: login <nome>");

  const r = await send("login", { user });
  console.log(r);

  if (r.data.status === "sucesso") {
    currentUser = user;

    // Sempre ouvir o próprio nome
    sub.subscribe(user);
    console.log(`📡 Agora ouvindo mensagens privadas em: ${user}`);
  }
}

async function cmdChannel(args) {
  const name = args[0];
  if (!name) return console.log("Uso: channel <nome>");

  const r = await send("channel", { name });
  console.log(r);
}

async function cmdChannels() {
  console.log(await send("channels"));
}

async function cmdUsers() {
  console.log(await send("users"));
}

async function cmdSubscribe(args) {
  const topic = args[0];
  if (!topic) return console.log("Uso: subscribe <canal>");

  sub.subscribe(topic);
  console.log(`📡 Agora ouvindo o tópico: ${topic}`);
}

async function cmdPublish(args) {
  if (!currentUser) return console.log("Faça login primeiro.");

  const channel = args[0];
  const message = args.slice(1).join(" ");

  if (!channel || !message)
    return console.log("Uso: publish <canal> <mensagem>");

  const r = await send("publish", {
    user: currentUser,
    channel,
    message,
  });

  console.log(r);
}

async function cmdMessage(args) {
  if (!currentUser) return console.log("Faça login primeiro.");

  const dst = args[0];
  const message = args.slice(1).join(" ");

  if (!dst || !message)
    return console.log("Uso: message <destino> <mensagem>");

  const r = await send("message", {
    src: currentUser,
    dst,
    message,
  });

  console.log(r);
}

// -------------------------------
// Tabela de comandos
// -------------------------------
const commands = {
  login: cmdLogin,
  channel: cmdChannel,
  channels: cmdChannels,
  users: cmdUsers,
  subscribe: cmdSubscribe,
  publish: cmdPublish,
  message: cmdMessage,
};

// -------------------------------
// REPL
// -------------------------------
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  prompt: "> ",
});

// -------------------------------
// Inicialização
// -------------------------------
(async () => {
  await req.connect(REQ_ADDR);
  console.log("📡 Conectado ao broker em", REQ_ADDR);

  await sub.connect(SUB_ADDR);
  console.log("📡 SUB conectado em", SUB_ADDR);

  startSubListener();

  rl.prompt();
  rl.on("line", async (line) => {
    const clean = line.replace(/\s+/g, " ").trim();
    if (!clean) {
      rl.prompt();
      return;
    }

    const tokens = clean.split(" ");
    const cmd = tokens[0].toLowerCase();
    const args = tokens.slice(1);

    if (!commands[cmd]) {
      console.log("Comando desconhecido.");
      rl.prompt();
      return;
    }

    try {
      await commands[cmd](args);
    } catch (e) {
      console.log("Erro:", e.message);
    }

    rl.prompt();
  });
})();
