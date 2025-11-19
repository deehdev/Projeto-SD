// =====================================================
// CLIENTE INTERATIVO — versão limpa sem autocomplete
// =====================================================

const zmq = require("zeromq");
const msgpack = require("@msgpack/msgpack");
const readline = require("readline");

// Relógio lógico
let clock = 0;
function incClock() { clock++; return clock; }
function updateClock(received) {
  received = Number(received) || 0;
  clock = Math.max(clock, received) + 1;
}

// Endereços
const REQ_ADDR = process.env.REQ_ADDR || "tcp://broker:5555";
const SUB_ADDR = process.env.SUB_ADDR || "tcp://proxy:5558";

// Sockets
const req = new zmq.Request();
const sub = new zmq.Subscriber();

let busy = false;
let currentUser = null;

// Lista local de inscrições
const subscriptions = new Set();

// Enviar request
async function send(service, data = {}) {
  if (busy) {
    console.log("⚠ Socket ocupado.");
    return;
  }

  busy = true;

  data.timestamp = new Date().toISOString();
  data.clock = incClock();

  await req.send(msgpack.encode({ service, data }));

  const reply = await req.receive();
  const decoded = msgpack.decode(reply[0]);

  updateClock(decoded.data.clock);

  busy = false;
  return decoded;
}

// -----------------------
// SUB Listener SEM AUTOCOMPLETE
// -----------------------
async function startSubListener() {
  for await (const [topicBuf, msgBuf] of sub) {
    try {
      const topic = topicBuf.toString().trim();
      const env = msgpack.decode(msgBuf);
      const data = env.data || {};
      const service = env.service;

      updateClock(data.clock);

      // imprime sem mexer no texto que o usuário digitou
      console.log("\n");
      if (service === "publish") {
        console.log(`💬  #${topic} | ${data.user} → ${data.message}`);
      } else if (service === "message") {
        console.log(`📩  ${data.src} → você | ${data.message}`);
      }
      console.log("");

      rl.prompt();

    } catch (e) {
      console.log("Erro SUB:", e);
    }
  }
}

// Comandos
async function cmdLogin(args) {
  const user = args[0];
  if (!user) return console.log("Uso: login <nome>");

  const r = await send("login", { user });
  console.log(r);

  if (r.data.status === "sucesso") {
    currentUser = user;

    sub.subscribe(user);
    subscriptions.add(user);
    console.log(`📡 ouvindo mensagens privadas em: ${user}`);
  }
}

async function cmdChannel(args) {
  const name = args[0];
  if (!name) return console.log("Uso: channel <nome>");
  console.log(await send("channel", { name }));
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
  subscriptions.add(topic);
  console.log(`📡 inscrito em: ${topic}`);
}

async function cmdPublish(args) {
  if (!currentUser) return console.log("Faça login primeiro.");

  const channel = args[0];
  const message = args.slice(1).join(" ");

  if (!channel || !message)
    return console.log("Uso: publish <canal> <mensagem>");

  if (!subscriptions.has(channel))
    return console.log(`🚫 você NÃO está inscrito no canal '${channel}'`);

  console.log(await send("publish", {
    user: currentUser, channel, message
  }));
}

async function cmdMessage(args) {
  if (!currentUser) return console.log("Faça login primeiro.");

  const dst = args[0];
  const message = args.slice(1).join(" ");

  if (!dst || !message)
    return console.log("Uso: message <destino> <mensagem>");

  console.log(await send("message", {
    src: currentUser, dst, message
  }));
}

// Tabela comandos
const commands = {
  login: cmdLogin,
  channel: cmdChannel,
  channels: cmdChannels,
  users: cmdUsers,
  subscribe: cmdSubscribe,
  publish: cmdPublish,
  message: cmdMessage,
};

// REPL sem autocomplete
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  prompt: "> ",
  terminal: true,
  historySize: 50,
});

// Inicialização
(async () => {
  await req.connect(REQ_ADDR);
  console.log("📡 conectou ao broker", REQ_ADDR);

  await sub.connect(SUB_ADDR);
  console.log("📡 SUB conectado em", SUB_ADDR);

  startSubListener();

  rl.prompt();
  rl.on("line", async (line) => {
    const clean = line.trim();
    if (!clean) return rl.prompt();

    const tokens = clean.split(" ");
    const cmd = tokens[0].toLowerCase();
    const args = tokens.slice(1);

    if (!commands[cmd]) {
      console.log("❓ comando desconhecido");
      return rl.prompt();
    }

    try {
      await commands[cmd](args);
    } catch (e) {
      console.log("Erro:", e.message);
    }

    rl.prompt();
  });
})();
