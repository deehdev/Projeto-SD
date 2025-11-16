import zmq
import json
import random
import threading
import time
from datetime import datetime, timezone

REQ_ADDR = "tcp://server:5555"
SUB_ADDR = "tcp://proxy:5558"

# Lock para sincronizar prints
print_lock = threading.Lock()

NOMES = [
    "Ana", "Maria", "Fernanda", "Lucia", "Juliana", "Paula", "Carla",
    "Bruno", "Carlos", "Eduardo", "Felipe", "Gustavo", "Henrique",
    "João", "Lucas", "Marcos", "Rafael", "Samuel", "Thiago", "Victor",
    "Elem", "Vini", "Iago", "Italo", "Luis", "Larissa", "Beatriz",
    "Sophia", "Helena", "Laura", "Isabela", "Camila", "Patricia",
    "Renata", "Leticia", "Bianca", "Nicole", "Marcela", "Gabriele",
    "Aline", "Tatiane", "Silvia", "Caroline", "Fabiana", "Nathalia",
    "Pedro", "André", "Leonardo", "Diego", "Rodrigo", "Caio", "Igor",
    "Fabrício", "Mauricio", "Daniel", "Matheus", "Vinicius", "Leandro",
    "Wesley", "Renan", "Hugo", "Arthur", "Enzo", "Davi", "Yuri",
    "Elaine", "Michele", "Simone", "Tainá", "Débora", "Yasmin",
    "Cecilia", "Joana", "Ester", "Manuela", "Elisa", "Melissa",
    "Veronica", "Adriana", "Sandra", "Mara", "Rebeca", "Carina",
    "Viviane", "Tereza", "Alex", "Jonathan", "Samuelson", "Cristiano",
    "Nilson", "Tales", "Paulo", "Jorge", "Adriel", "Otavio", "Alan",
    "Christian", "Rogério", "Nelson", "Fernando", "Luciano", "Sergio",
    "Everton", "Edson", "Valter"
]   #  <-- ESTA LINHA FALTAVA

# ---------------------------------------------------
# NOME REAL SEM REPETIÇÃO
# ---------------------------------------------------
def random_name():
    ctx = zmq.Context()
    req = ctx.socket(zmq.REQ)
    req.connect(REQ_ADDR)

    req.send_json({"service": "users", "data": {}})
    usados = [u.lower() for u in req.recv_json()["data"]["users"]]
    req.close()

    disponiveis = [n for n in NOMES if n.lower() not in usados]

    if not disponiveis:
        print("\n❌ SEM NOMES DISPONÍVEIS — TODOS EM USO!")
        exit(1)

    return random.choice(disponiveis)

def now_iso():
    return datetime.now(timezone.utc).isoformat()

def now_time():
    return datetime.now().strftime("%H:%M:%S")

def log_print(msg):
    with print_lock:
        print(f"[{now_time()}] {msg}", flush=True)

def send_request(sock, service, data):
    sock.send_json({"service": service, "data": data})
    return sock.recv_json()

# ---------------------------------------------------
# HEARTBEAT
# ---------------------------------------------------
def send_heartbeat(username):
    ctx = zmq.Context()
    hb = ctx.socket(zmq.REQ)
    hb.connect(REQ_ADDR)

    while True:
        try:
            hb.send_json({"service": "heartbeat", "data": {"user": username}})
            hb.recv_json()
        except:
            pass
        time.sleep(5)

# ---------------------------------------------------
# RESPOSTAS ALEATÓRIAS
# ---------------------------------------------------
def gerar_resposta(nome):
    frases = [
        "Alguém viu algum filme bom ultimamente?", "Tô procurando algo leve pra assistir hoje.",
        "Esse mês saiu muito filme bom.", "Vocês gostam mais de filme dublado ou legendado?",
        "Tô com vontade de rever um clássico.", "Qual foi o último filme que te surpreendeu?",
        "Preciso de uma recomendação urgente.", "Eu sempre paro no meio do filme e volto depois.",
        "Tem filme que nunca envelhece.", "Hoje tô com vontade de ver terror.",
        "Eu fico preso em trilogia fácil demais.", "Vi um filme estranho ontem, mas gostei.",
        "Tem filme que eu assisto mil vezes sem cansar.", "Eu adoro quando um filme tem um final inesperado.",
        "Filme bom é aquele que faz pensar depois.", "Faz tempo que não vejo um suspense decente.",
        "Queria saber qual filme está bombando agora.", "Nada supera assistir filme comendo pipoca.",
        "Eu sempre gosto mais do trailer que do filme.", "Tem ator que melhora qualquer produção.",
        "Eu gosto de ver filmes antes de dormir.", "Às vezes só quero um filme bobo pra relaxar.",
        "Tem filme que devia ter continuação.", "E tem filme que devia ter parado no primeiro.",
        "Assisti um filme aleatório e adorei.", "Eu fico perdido em filme com muita explicação.",
        "A última cena do filme que vi foi perfeita.", "Eu adoro filmes com trilha sonora boa.",
        "Quem aí curte filmes baseados em fatos reais?", "Eu gosto de filme curto, sem enrolação.",
        "Filme longo só funciona se for muito bom.", "Sinto falta de filmes antigos passando na TV.",
        "Qual o melhor filme que você já viu?", "Hoje acordei com vontade de ver comédia.",
        "Tem diretor que eu vejo tudo que lança.", "A fotografia de alguns filmes é absurda.",
        "Cansado de escolher filme e não assistir nada.", "Eu queria ver algo diferente hoje.",
        "Sempre que termino um filme vou pesquisar sobre ele.",
        "Filme ruim deixa o dia inteiro pior kkkkk",
    ]
    return random.choice(frases)

# ---------------------------------------------------
# ESCUTA SUB (RECEPÇÃO)
# ---------------------------------------------------
def listen_messages(username, sub_socket):
    while True:
        try:
            topic = sub_socket.recv_string()
            msg = sub_socket.recv_string()

            # =============================
            # MENSAGEM PRIVADA
            # =============================
            if topic.lower() == username.lower():
                try:
                    data = json.loads(msg)
                    src = data.get("src", "???")
                    texto = data.get("message", msg)
                    log_print(f"💌 {username} recebeu mensagem PRIVADA de {src}: {texto}")
                except:
                    log_print(f"💌 {username} recebeu mensagem PRIVADA: {msg}")
                continue

            # =============================
            # MENSAGEM PÚBLICA
            # =============================
            try:
                data = json.loads(msg)
                src = data.get("user", "???")
                texto = data.get("message", msg)

                if src.lower() != username.lower():
                    log_print(f"💬 [{src}] em #{topic}: {texto}")
            except:
                pass

        except Exception as e:
            log_print(f"❌ [{username}] ERRO lendo mensagem: {e}")
            time.sleep(1)

# ---------------------------------------------------
# BOT PRINCIPAL
# ---------------------------------------------------
def main():
    username = random_name()

    with print_lock:
        print(f"\n{'='*60}")
        print(f"🤖 BOT INICIADO: {username}")
        print(f"{'='*60}\n")

    ctx = zmq.Context()

    req = ctx.socket(zmq.REQ)
    req.connect(REQ_ADDR)

    sub = ctx.socket(zmq.SUB)
    sub.connect(SUB_ADDR)

    # LOGIN
    resp = send_request(req, "login", {"user": username, "timestamp": now_iso()})
    log_print(f"✅ [{username}] LOGIN: {resp['data']['status']}")

    # HEARTBEAT
    threading.Thread(target=send_heartbeat, args=(username,), daemon=True).start()

    # CANAIS EXISTENTES
    resp_channels = send_request(req, "channels", {})
    channels = resp_channels.get("data", {}).get("channels", [])

    if not channels:
        canal_novo = "Geral"
        send_request(req, "channel", {"channel": canal_novo})
        channels = [canal_novo]
        log_print(f"📡 [{username}] CRIOU o canal '{canal_novo}'.")

    # Escolhe salas
    salas = random.sample(channels, random.randint(1, min(3, len(channels))))
    canal_privado = username.lower()

    log_print(f"📺 [{username}] INSCRITO NOS CANAIS: {', '.join(salas)}")
    log_print(f"🔒 [{username}] INBOX PRIVADA: {canal_privado}")

    # Registrar no servidor + SUBSCRIBE
    for ch in salas:
        sub.setsockopt_string(zmq.SUBSCRIBE, ch)
        send_request(req, "subscribe", {"user": username, "channel": ch})

    sub.setsockopt_string(zmq.SUBSCRIBE, canal_privado)
    send_request(req, "subscribe", {"user": username, "channel": canal_privado})

    log_print(f"{'-'*60}")

    # Iniciar thread de recepção
    threading.Thread(target=listen_messages, args=(username, sub), daemon=True).start()

    # LOOP PRINCIPAL
    while True:
        time.sleep(random.uniform(4, 8))
        modo = random.choice(["publica", "privada"])

        # ----------------------------
        # MENSAGEM PÚBLICA
        # ----------------------------
        if modo == "publica":
            canal = random.choice(salas)
            texto = gerar_resposta(username)

            send_request(req, "publish", {
                "user": username,
                "channel": canal,
                "message": texto,
                "timestamp": now_iso()
            })

            log_print(f"📢 [{username}] publicou em #{canal}: {texto}")

        # ----------------------------
        # MENSAGEM PRIVADA
        # ----------------------------
        else:
            try:
                users_resp = send_request(req, "users", {})
                users_online = [u.lower() for u in users_resp["data"]["users"] if isinstance(u, str)]
                online_validos = [u for u in users_online if u != username.lower()]

                if not online_validos:
                    log_print(f"⚠️ [{username}] Não há outros usuários online.")
                    continue

                destino = random.choice(online_validos)
                texto = gerar_resposta(username)

                resp_envio = send_request(req, "message", {
                    "src": username,
                    "dst": destino,
                    "message": texto,
                    "timestamp": now_iso()
                })

                status = resp_envio.get("data", {}).get("status", "").lower()

                if status not in ["ok", "sucesso", "success"]:
                    log_print(f"❌ [{username}] Usuário '{destino}' offline.")
                    continue

                log_print(f"📤 {username} enviou mensagem PRIVADA para {destino}: {texto}")
                log_print(f"📨 {destino} recebeu mensagem PRIVADA de {username}: {texto}")

            except Exception as e:
                log_print(f"❌ [{username}] ERRO ao enviar PM: {e}")

if __name__ == "__main__":
    main()