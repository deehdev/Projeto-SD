#!/usr/bin/env python3
import os 
import zmq
import msgpack
import random
import threading
import time
from datetime import datetime, timezone
import sys
sys.stdout.reconfigure(line_buffering=True)

# ---------------------------------------------------
# CONFIG
# ---------------------------------------------------
REQ_ADDR = os.environ.get("REQ_ADDR", "tcp://broker:5555")
SUB_ADDR = os.environ.get("SUB_ADDR", "tcp://proxy:5558")

# ---------------------------------------------------
# RELÓGIO LÓGICO
# ---------------------------------------------------
logical_clock = 0
clock_lock = threading.Lock()

def inc_clock():
    global logical_clock
    with clock_lock:
        logical_clock += 1
        return logical_clock

def update_clock(recv):
    global logical_clock
    with clock_lock:
        try:
            rc = int(recv)
            logical_clock = max(logical_clock, rc) + 1
        except:
            logical_clock += 1
        return logical_clock

def now_iso():
    return datetime.now(timezone.utc).isoformat()

# ---------------------------------------------------
# REQUEST (REQ → REP)
# ---------------------------------------------------
def send_req(sock, service, data=None, timeout=5.0):
    if data is None:
        data = {}

    env = {
        "service": service,
        "data": data,
        "timestamp": now_iso(),
        "clock": inc_clock()
    }

    encoded = msgpack.packb(env, use_bin_type=True)
    try:
        sock.send(encoded)
    except Exception as e:
        return {"service":"error","data":{"status":str(e)}}

    try:
        poller = zmq.Poller()
        poller.register(sock, zmq.POLLIN)
        socks = dict(poller.poll(int(timeout*1000)))
        if socks.get(sock) == zmq.POLLIN:
            raw = sock.recv()
            reply = msgpack.unpackb(raw, raw=False)
            update_clock(reply.get("clock", 0))
            return reply
        else:
            return {"service":"error","data":{"status":"timeout"}}
    except Exception as e:
        return {"service":"error","data":{"status":str(e)}}

# ---------------------------------------------------
# SUB LISTENER
# ---------------------------------------------------
def sub_listener(sub):
    while True:
        try:
            parts = sub.recv_multipart()
            if len(parts) < 2:
                continue

            topic = parts[0].decode('utf-8', errors='ignore')
            env = msgpack.unpackb(parts[1], raw=False)
            update_clock(env.get("clock", 0))

            svc = env.get("service", "")
            data = env.get("data", {})
            ts = env.get("timestamp", "sem-timestamp")
            clk = env.get("clock", "?")

            # mensagens de canal
            if svc == "publish":
                user = data.get("user") or data.get("src") or "?"
                message = data.get("message") or data.get("msg") or ""

                print(
                    f"[{topic}] {user}: {message}  "
                    f"(timestamp={ts}, clock={clk})"
                )

            # mensagens privadas
            elif svc == "message":
                src = data.get("src") or data.get("user") or "?"
                dst = topic
                message = data.get("message") or data.get("msg") or ""

                print(
                    f"📩 {dst} recebeu mensagem PRIVADA de {src}: {message}  "
                    f"(timestamp={ts}, clock={clk})"
                )

            # outros tópicos (election, replicate…)
            else:
                print(f"[{topic}][{svc}] {data}")

        except Exception as e:
            print("Erro no SUB:", e)
            time.sleep(0.5)

# ---------------------------------------------------
# HEARTBEAT
# ---------------------------------------------------
def heartbeat(username):
    ctx = zmq.Context()
    hb = ctx.socket(zmq.REQ)
    hb.setsockopt(zmq.LINGER, 0)
    hb.connect(REQ_ADDR)
    time.sleep(0.05)

    while True:
        try:
            send_req(hb, "heartbeat", {"user": username}, timeout=2.0)
        except Exception:
            pass
        time.sleep(5)

# ---------------------------------------------------
# FRASES / NOMES
# ---------------------------------------------------
def frase():
    frases = [
        "Alguém viu algum filme bom?",
        "Preciso de uma recomendação urgente.",
        "Esse mês saiu muito filme bom!",
        "Vocês preferem dublado ou legendado?",
        "Interstellar é perfeito.",
        "Quero algo leve!",
        "Alguém entendeu Tenet?",
        "Recomendações de terror psicológico?"
    ]
    return random.choice(frases)

NOMES = [
    "Ana","Pedro","Rafael","Deise","Camila","Victor",
    "Paula","Juliana","Lucas","Marcos","Mateus","João",
    "Carla","Bruno","Renata","Sofia"
]

# ---------------------------------------------------
# BOT
# ---------------------------------------------------
def main():
    username = random.choice(NOMES)
    print(f"\nBOT iniciado como {username}\n")

    ctx = zmq.Context()
   
    req = ctx.socket(zmq.REQ)
    req.setsockopt(zmq.LINGER, 0)
    req.connect(REQ_ADDR)
    time.sleep(0.05)

    sub = ctx.socket(zmq.SUB)
    sub.setsockopt(zmq.LINGER, 0)
    sub.connect(SUB_ADDR)
    time.sleep(0.05)

    # LOGIN
    r = send_req(req, "login", {"user": username})
    print("LOGIN:", r.get("data", {}).get("status", "erro"))

    # PRIVATE TOPIC
    sub.setsockopt_string(zmq.SUBSCRIBE, username)

    threading.Thread(target=heartbeat, args=(username,), daemon=True).start()

    # CANAIS
    r = send_req(req, "channels")
    channels = r.get("data", {}).get("channels", [])

    if not channels:
        send_req(req, "channel", {"channel": "geral"})
        channels = ["geral"]

    salas = random.sample(channels, 1)

    for c in salas:
        sub.setsockopt_string(zmq.SUBSCRIBE, c)
        send_req(req, "subscribe", {"user": username, "topic": c})

    print("Canais inscritos:", salas)

    threading.Thread(target=sub_listener, args=(sub,), daemon=True).start()

    # LOOP PRINCIPAL
    while True:

        # 40% chance → PRIVADA
        if random.random() < 0.40:
            dst = random.choice([n for n in NOMES if n != username])
            text = frase()

            send_req(req, "message", {
                "src": username,
                "dst": dst,
                "message": text
            })

            print(f"💌 {username} enviou mensagem Privada para {dst}: {text}")

        # 60% chance → PUBLICAÇÃO
        else:
            canal = random.choice(salas)
            text = frase()

            send_req(req, "publish", {
                "user": username,
                "channel": canal,
                "message": text
            })

            print(f"[{canal}] {username}: {text}")

        time.sleep(random.uniform(3, 6))


if __name__ == "__main__":
    main()
