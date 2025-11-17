// client/client.js
const zmq = require("zeromq");
const msgpack = require("@msgpack/msgpack");
const readline = require("readline");

let logicalClock = 0;
let currentUser = null;

function incClock() {
  return ++logicalClock;
}

function updateClock(recv) {
  logicalClock = Math.max(logicalClock, recv) + 1;
}

// CONFIG
const BROKER_REQ = "tcp://broker:5555";
const PROXY_SUB = "tcp://proxy:5558";

const req = new zmq.Request();
const sub = new zmq.Subscriber();

let busy = false;

// ======================
// START
// ======================
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

// ======================
// SUB LOOP
// ======================
async function startSubLoop() {
  for await (const [topic, payload] of sub) {
    try {
      const env = msgpack.decode(payload);
      updateClock(env.clock);

      console.log(
        `\n[SUB:${topic}] clock=${env.clock} → local=${logicalClock}`,
        env.data
      );

      process.stdout.write("> ");
    } catch (err) {
      console.log("[SUB ERRO]", err);
    }
  }
}

// ======================
// SEND (REQ/REP)
// ======================
async function send(service, data = {}) {
  if (busy) {
    console.log("⏳ Aguarde a requisição anterior terminar.");
    return;
  }
  busy = true;

  try {
    // CORRECT STRUCTURE
    const envelope = {
      service,
      data: {
        ...data,
        timestamp: new Date().toISOString()
      },
      timestamp: new Date().toISOString(),
      clock: incClock()
    };

    const encoded = msgpack.encode(envelope);
    await req.send(encoded);

    const [replyBytes] = await req.receive();
    const reply = msgpack.decode(replyBytes);

    updateClock(reply.clock);

    console.log(
      `[REQ:${service}] local=${logicalClock} ← reply_clock=${reply.clock}`,
      reply.data
    );

    // Save logged in user
    if (service === "login" && reply.data?.user) {
      currentUser = reply.data.user;

      sub.subscribe(currentUser);
      console.log(`📌 Inscrito no tópico privado: ${currentUser}`);
    }

    return reply;

  } catch (err) {
    console.log("[REQ ERRO]", err);
  } finally {
    busy = false;
  }
}

// ======================
// CLI INPUT
// ======================
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout
});

function startInputLoop() {
  rl.setPrompt("> ");
  rl.prompt();

  rl.on("line", async (line) => {
    const parts = line.trim().split(" ");
    const cmd = parts[0];

    try {
      switch (cmd) {
        case "login":
          await send("login", { user: parts[1] });
          break;

        case "users":
          await send("users");
          break;

        case "channels":
          await send("channels");
          break;

        case "channel":
          await send("channel", { channel: parts[1] });
          break;

        case "publish":
          await send("publish", {
            user: currentUser,
            channel: parts[1],
            message: parts.slice(2).join(" ")
          });
          break;

        case "message":
          await send("message", {
            src: currentUser,
            dst: parts[1],
            message: parts.slice(2).join(" ")
          });
          break;

        case "subscribe":
          sub.subscribe(parts[1]);
          await send("subscribe", { user: currentUser, topic: parts[1] });
          break;

        case "unsubscribe":
          sub.unsubscribe(parts[1]);
          await send("unsubscribe", { user: currentUser, topic: parts[1] });
          break;

        default:
          console.log("Comando desconhecido.");
      }
    } catch (err) {
      console.log("[CLIENT ERRO]", err);
    }

    rl.prompt();
  });
}
