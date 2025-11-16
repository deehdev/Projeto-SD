const zmq = require("zeromq");
const readline = require("readline");

function nowISO() {
  return new Date().toISOString();
}

async function sendRequest(msg) {
  const req = new zmq.Request();
  await req.connect("tcp://server:5555");
  await req.send(JSON.stringify(msg));
  const [reply] = await req.receive();
  return reply.toString();
}

async function main() {
  console.log("Connected to server");
  console.log("Commands:");
  console.log(" login <user>");
  console.log(" users");
  console.log(" channel <name>");
  console.log(" channels");      // 👈 novo comando
  console.log(" exit\n");

  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
    prompt: "> "
  });

  rl.prompt();

  for await (const line of rl) {
    const parts = line.trim().split(" ");
    const cmd = parts[0];

    if (cmd === "exit") process.exit(0);

    let msg = null;

    if (cmd === "login") {
      const user = parts[1] || "anonymous";
      msg = { service: "login", data: { user, timestamp: nowISO() } };

    } else if (cmd === "users") {
      msg = { service: "users", data: { timestamp: nowISO() } };

    } else if (cmd === "channel") {
      const channel = parts[1] || "general";
      msg = { service: "channel", data: { channel, timestamp: nowISO() } };

    } else if (cmd === "channels") {
      msg = { service: "channels", data: { timestamp: nowISO() } };  // 👈 nova rota

    } else {
      console.log("Unknown command");
      rl.prompt();
      continue;
    }

    try {
      const reply = await sendRequest(msg);
      console.log("Reply:", reply);
    } catch (err) {
      console.error("ERROR:", err);
    }

    rl.prompt();
  }
}

main();