// ==============================
//  TUI Client para ZMQ + MessagePack
// ==============================

const zmq = require("zeromq");
const msgpack = require("@msgpack/msgpack");
const blessed = require("blessed");

// guarda canais inscritos
const subscribed = new Set();

// CONFIG (corrigido!)
const REQ_ADDR = "tcp://broker:5555";   // correto no docker-compose
const SUB_ADDR = "tcp://proxy:5558";    // XPUB do proxy

let username = "tui_" + Math.floor(Math.random() * 9999);

// ==============================
//  TUI SETUP
// ==============================
const screen = blessed.screen({
    smartCSR: true,
    title: "Painel TUI - Sistema Distribuído"
});

const messagesBox = blessed.box({
    top: 0, left: 0,
    width: "70%", height: "70%",
    label: " 📩 Mensagens ",
    border: "line",
    scrollable: true,
    alwaysScroll: true,
    scrollbar: { ch: " ", track: { bg: "gray" }, style: { bg: "white" } }
});

const infoBox = blessed.box({
    top: 0, left: "70%",
    width: "30%", height: "40%",
    label: " 🧠 Sistema ",
    border: "line"
});

const channelsBox = blessed.box({
    top: "40%", left: "70%",
    width: "30%", height: "30%",
    label: " 📡 Canais ",
    border: "line"
});

const input = blessed.textbox({
    bottom: 0, left: 0,
    width: "100%", height: 3,
    border: "line",
    label: " ✏ Enviar mensagem ",
    inputOnFocus: true
});

screen.append(messagesBox);
screen.append(infoBox);
screen.append(channelsBox);
screen.append(input);

// exit
screen.key(['C-c'], () => process.exit(0));

function logMessage(msg) {
    messagesBox.insertBottom(msg);
    messagesBox.setScrollPerc(100);
    screen.render();
}

function logInfo(msg) {
    infoBox.setContent(msg);
    screen.render();
}

function updateChannels(chList) {
    channelsBox.setContent(chList.join("\n"));
    screen.render();
}

// ==============================
//  ZMQ SOCKETS
// ==============================
const req = new zmq.Request();
const sub = new zmq.Subscriber();

// conectar
(async () => {
    await req.connect(REQ_ADDR);
    await sub.connect(SUB_ADDR);

    sub.subscribe(""); // recebe tudo (mas filtramos manualmente)
    logInfo("Conectado.\nREQ:" + REQ_ADDR + "\nSUB:" + SUB_ADDR);

    // login automático
    await sendReq("login", { user: username });
    logMessage("[LOGADO] como " + username);

    // pedir lista de canais
    const ch = await sendReq("channels", {});
    updateChannels(ch.data.channels);
})();

// ==============================
//  Função para enviar mensagens
// ==============================
async function sendReq(service, data) {
    const env = {
        service: service,
        data: data,
        timestamp: new Date().toISOString(),
        clock: 0
    };

    const buf = msgpack.encode(env);
    await req.send(buf);
    const [reply] = await req.receive();

    try {
        return msgpack.decode(reply);
    } catch {
        return {};
    }
}

// ==============================
//  SUB listener
// ==============================
(async () => {
    for await (const [topic, payload] of sub) {
        let env;

        try { env = msgpack.decode(payload); }
        catch { continue; }

        const t = topic.toString();

        // mensagens replicadas (logs entre servidores)
        if (t === "replicate") {
            logMessage("🔁 REP: " + JSON.stringify(env.data));
        }

        // mudança de coordenador
        if (t === "servers") {
            logInfo("Coordenador → " + env.data.coordinator);
        }

        // mensagens normais de canal (usuário ✉)
        if (!["replicate", "servers"].includes(t)) {
            logMessage(`📨 [${t}] ${JSON.stringify(env.data)}`);
        }
    }
})();

// ==============================
//  INPUT — enviar mensagens
// ==============================
input.on("submit", async (text) => {
    if (text.trim().length === 0) return;

    const parts = text.split(" ");
    const channel = parts[0];
    const content = parts.slice(1).join(" ");

    // 🔥 AUTO-INSCRIÇÃO NO CANAL
    if (!subscribed.has(channel)) {
        sub.subscribe(channel);
        subscribed.add(channel);
        logMessage(`📡 Inscrito no canal: ${channel}`);
    }

    // 🔥 ENVIA PARA O SERVIDOR
    await sendReq("publish", {
        channel: channel,
        user: username,
        message: content
    });

    // feedback na tela
    logMessage(`📤 (${channel}) ${username}: ${content}`);

    input.clearValue();
    screen.render();
    input.focus();
});

input.focus();
screen.render();
