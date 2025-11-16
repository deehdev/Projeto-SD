
const zmq = require("zeromq");
const readline = require("readline");

function nowISO() {
  return new Date().toISOString();
}

// -----------------------------------
// REQ para o servidor Go
// -----------------------------------
async function sendRequest(msg) {
  const req = new zmq.Request();
  await req.connect("tcp://server:5555");
  await req.send(JSON.stringify(msg));

  const [reply] = await req.receive();
  return JSON.parse(reply.toString());
}

// -----------------------------------
// MAIN
// -----------------------------------
async function main() {
  console.log("Connected to server");
  console.log("Commands:");
  console.log(" login <user>");
  console.log(" users");
  console.log(" channels");
  console.log(" channel <name>");
  console.log(" publish <channel> <msg>");
  console.log(" message <user> <msg>");
  console.log(" subscribe <topic>");
  console.log(" unsubscribe <topic>");
  console.log(" mysubs");
  console.log(" publishable");
  console.log(" exit\n");

  // -----------------------------------
  // SUB socket
  // -----------------------------------
  const sub = new zmq.Subscriber();
  await sub.connect("tcp://proxy:5558");

  const localSubs = new Set();
  let listening = false;

  // -----------------------------------
  // LISTENER QUE NÃO ATRAPALHA A DIGITAÇÃO
  // -----------------------------------
  async function startListener() {
    if (listening) return;
    listening = true;

    console.log("[LISTEN] Ouvindo mensagens...");

    for await (const [topicBuf, msgBuf] of sub) {
      const topic = topicBuf.toString();
      const msg = msgBuf.toString();

      const currentInput = rl.line;

      readline.clearLine(process.stdout, 0);
      readline.cursorTo(process.stdout, 0);

      console.log(`\n[RECEBIDO] ${topic} → ${msg}`);

      process.stdout.write("> " + currentInput);
    }
  } //  ← FECHA startListener

  // -----------------------------------
  // CLI
  // -----------------------------------
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
    prompt: "> "
  });

  rl.prompt();

  for await (const line of rl) {
    const parts = line.trim().split(" ");
    const cmd = parts[0];
    let msg = null;

    if (cmd === "exit") {
      console.log("Bye!");
      process.exit(0);
    }

    // -----------------------------
    // SUBSCRIBE
    // -----------------------------
    if (cmd === "subscribe") {
      let topic = parts[1];
      if (!topic) {
        console.log("usage: está inscrito no canal <channel>");
        rl.prompt();
        continue;
      }

      topic = topic.trim().toLowerCase();

      const reply = await sendRequest({
        service: "subscribe",
        data: { user: loggedUser, channel: topic }
      });

      if (reply.data.status === "sucesso") {
        sub.subscribe(topic);
        localSubs.add(topic);
        console.log("[OK] inscrito:", topic);
      } else {
        console.log("[ERRO]", reply.data.message);
      }

      rl.prompt();
      continue;
    }

    // -----------------------------
    // UNSUBSCRIBE
    // -----------------------------
    if (cmd === "unsubscribe") {
      let topic = parts[1];
      if (!topic) {
        console.log("usage: saiu do canal <channel>");
        rl.prompt();
        continue;
      }

      topic = topic.trim().toLowerCase();

      const reply = await sendRequest({
        service: "unsubscribe",
        data: { user: loggedUser, channel: topic }
      });

      if (reply.data.status === "sucesso") {
        sub.unsubscribe(topic);
        localSubs.delete(topic);
        console.log("[OK] saiu do canal ", topic);
      } else {
        console.log("[ERRO]", reply.data.message);
      }

      rl.prompt();
      continue;
    }

    // -----------------------------
    // MYSUBS
    // -----------------------------
    if (cmd === "mysubs") {
      const reply = await sendRequest({
        service: "mysubs",
        data: { user: loggedUser }
      });

      console.log(reply.data);
      rl.prompt();
      continue;
    }

    // -----------------------------
    // PUBLISH COM BLOQUEIO
    // -----------------------------
    if (cmd === "publish") {
      if (!loggedUser) {
        console.log("[ERRO] você precisa fazer login primeiro");
        rl.prompt();
        continue;
      }

      const canal = parts[1]?.toLowerCase();
      const texto = parts.slice(2).join(" ");

      if (!canal || !texto) {
        console.log("uso: publish <canal> <mensagem>");
        rl.prompt();
        continue;
      }

      if (!localSubs.has(canal)) {
        console.log(`[ERRO] você NÃO está inscrito no canal '${canal}'`);
        rl.prompt();
        continue;
      }

      msg = {
        service: "publish",
        data: {
          user: loggedUser,
          channel: canal,
          message: texto,
          timestamp: nowISO()
        }
      };
    }

    // -----------------------------
    // LOGIN + AUTO LISTENER
    // -----------------------------
    if (cmd === "login") {
      const user = parts[1];

      const reply = await sendRequest({
        service: "login",
        data: { user, timestamp: nowISO() }
      });

      console.log("Reply:", reply);

      if (reply.data.status === "sucesso") {
        loggedUser = user;

        sub.subscribe(user);
        localSubs.add(user);
        console.log("[OK] inbox privada assinada:", user);

        startListener();
      }

      rl.prompt();
      continue;
    }

    // -----------------------------
    // OUTROS COMANDOS
    // -----------------------------
    if (cmd === "users") msg = { service: "users", data: {} };
    if (cmd === "channels") msg = { service: "channels", data: {} };
    if (cmd === "channel") msg = { service: "channel", data: { channel: parts[1] } };

    if (cmd === "message") {
      msg = {
        service: "message",
        data: {
          src: loggedUser,
          dst: parts[1],
          message: parts.slice(2).join(" "),
          timestamp: nowISO()
        }
      };
    }

    // -----------------------------
    // PUBLISHABLE (NOVO COMANDO)
    // -----------------------------
    if (cmd === "publishable") {
      if (!loggedUser) {
        console.log("[ERRO] você precisa fazer login primeiro");
        rl.prompt();
        continue;
      }

      msg = {
        service: "publishable",
        data: {
          user: loggedUser
        }
      };
    }

    if (!msg) {
      console.log("Unknown command");
      rl.prompt();
      continue;
    }


    if (!msg) {
      console.log("Unknown command");
      rl.prompt();
      continue;
    }
    
    // EXECUTE REQUEST
    try {
      const reply = await sendRequest(msg);
      console.log("Reply:", reply);
    } catch (err) {
      console.error("ERROR:", err);
    }

    rl.prompt();
  }
}

let loggedUser = null;
main();
