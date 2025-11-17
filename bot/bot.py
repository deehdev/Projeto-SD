#!/usr/bin/env python3
import zmq
import msgpack
import random
import threading
import time
from datetime import datetime, timezone

# ---------------------------------------------------
# CONFIG
# ---------------------------------------------------
REQ_ADDR = "tcp://broker:5555"
SUB_ADDR = "tcp://proxy:5558"

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
            pass
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
    sock.send(encoded)

    # esperar reply
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
# SUB LISTENER (trata multipart [topic, payload])
# ---------------------------------------------------
def sub_listener(sub):
    while True:
        try:
            parts = sub.recv_multipart()
            if len(parts) < 2:
                continue
            topic = parts[0].decode('utf-8', errors='ignore')
            payload = parts[1]
            env = msgpack.unpackb(payload, raw=False)
            update_clock(env.get("clock", 0))

            svc = env.get("service", "")
            data = env.get("data", {})

            # mensagens de publicação em canal
            if svc == "publish":
                user = data.get("user") or data.get("src") or "?"
                message = data.get("message") or data.get("msg") or ""
                print(f"[{topic}] {user}: {message}")

            # mensagens privadas (tópico = username)
            elif svc == "message":
                src = data.get("src") or data.get("user") or "?"
                message = data.get("message") or data.get("msg") or ""
                print(f"💌 PRIVADA de {src}: {message}")

            # replicação / servidores
            else:
                print(f"[{topic}][{svc}] {data}")

        except Exception as e:
            print("Erro no SUB:", e)
            time.sleep(0.5)

# ---------------------------------------------------
# HEARTBEAT (opcional)
# ---------------------------------------------------
def heartbeat(username):
    ctx = zmq.Context()
    hb = ctx.socket(zmq.REQ)
    hb.connect(REQ_ADDR)
    while True:
        try:
            send_req(hb, "heartbeat", {"user": username})
        except:
            pass
        time.sleep(5)

# ---------------------------------------------------
# Frases e nomes
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

NOMES = ["Ana","Pedro","Rafael","Deise","Camila","Victor","Paula","Juliana","Lucas","Marcos","Mateus","João","Carla","Bruno","Renata","Sofia"]

# ---------------------------------------------------
# BOT
# ---------------------------------------------------
def main():
    username = random.choice(NOMES)
    print(f"\nBOT iniciado como {username}\n")

    ctx = zmq.Context()
    req = ctx.socket(zmq.REQ)
    req.connect(REQ_ADDR)

    sub = ctx.socket(zmq.SUB)
    sub.connect(SUB_ADDR)

    # login
    r = send_req(req, "login", {"user": username})
    status = r.get("data", {}).get("status", "erro")
    print("LOGIN:", status)

    # subscreve tópicos privados e canais
    sub.setsockopt_string(zmq.SUBSCRIBE, username)

    # heartbeat (opcional)
    threading.Thread(target=heartbeat, args=(username,), daemon=True).start()

    # lista canais
    r = send_req(req, "channels")
    channels = r.get("data", {}).get("channels", [])
    if not channels:
        # observe: servidor espera key "channel"
        send_req(req, "channel", {"channel": "geral"})
        channels = ["geral"]

    # escolhe subscrições e solicita ao servidor subscribe
    salas = random.sample(channels, k=max(1, min(len(channels), 1)))
    for c in salas:
        sub.setsockopt_string(zmq.SUBSCRIBE, c)
        send_req(req, "subscribe", {"user": username, "topic": c})

    print("Canais inscritos:", salas)

    # start sub listener
    threading.Thread(target=sub_listener, args=(sub,), daemon=True).start()

    # loop publica
    while True:
        canal = random.choice(salas)
        text = frase()
        send_req(req, "publish", {"user": username, "channel": canal, "message": text})
        print(f"[{canal}] {username}: {text}")
        time.sleep(random.uniform(3, 6))

if __name__ == "__main__":
    main()
