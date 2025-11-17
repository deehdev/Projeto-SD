import zmq
import msgpack
import random
import threading
import time
from datetime import datetime, timezone

REQ_ADDR = "tcp://server:5555"
SUB_ADDR = "tcp://proxy:5558"

# Lock para sincronizar prints
print_lock = threading.Lock()

NOMES = [
    "Ana","Maria","Fernanda","Lucia","Juliana","Paula","Carla","Bruno","Carlos","Eduardo",
    "Felipe","Gustavo","Henrique","João","Lucas","Marcos","Rafael","Samuel","Thiago","Victor",
    "Elem","Vini","Iago","Italo","Luis","Larissa","Beatriz","Sophia","Helena","Laura","Isabela",
    "Camila","Patricia","Renata","Leticia","Bianca","Nicole","Marcela","Gabriele","Aline","Tatiane",
    "Silvia","Caroline","Fabiana","Nathalia","Pedro","André","Leonardo","Diego","Rodrigo","Caio",
    "Igor","Fabrício","Mauricio","Daniel","Matheus","Vinicius","Leandro","Wesley","Renan","Hugo",
    "Arthur","Enzo","Davi","Yuri","Elaine","Michele","Simone","Tainá","Débora","Yasmin","Cecilia",
    "Joana","Ester","Manuela","Elisa","Melissa","Veronica","Adriana","Sandra","Mara","Rebeca",
    "Carina","Viviane","Tereza","Alex","Jonathan","Samuelson","Cristiano","Nilson","Tales","Paulo",
    "Jorge","Adriel","Otavio","Alan","Christian","Rogério","Nelson","Fernando","Luciano","Sergio",
    "Everton","Edson","Valter"
]

# ---------------------------------------------------
# Nome único usando MessagePack
# ---------------------------------------------------
def random_name():
    ctx = zmq.Context()
    req = ctx.socket(zmq.REQ)
    req.connect(REQ_ADDR)

    req.send(msgpack.packb({"service":"users","data":{}}, use_bin_type=True))
    raw = req.recv()
    resp = msgpack.unpackb(raw, raw=False)
    req.close()

    usados = resp.get("data", {}).get("users", [])
    usados = [u.lower() for u in usados]

    disponiveis = [n for n in NOMES if n.lower() not in usados]

    if not disponiveis:
        print("\n❌ SEM NOMES DISPONÍVEIS!")
        exit(1)

    return random.choice(disponiveis)

def now_iso():
    return datetime.now(timezone.utc).isoformat()

def now_time():
    return datetime.now().strftime("%H:%M:%S")

def log_print(msg, user=None):
    with print_lock:
        print(msg, flush=True)

# ---------------------------------------------------
# REQ MessagePack helper
# ---------------------------------------------------
def send_request(sock, service, data):
    payload = {"service": service, "data": data}
    sock.send(msgpack.packb(payload, use_bin_type=True))
    return msgpack.unpackb(sock.recv(), raw=False)

# ---------------------------------------------------
# Heartbeat
# ---------------------------------------------------
def send_heartbeat(username):
    ctx = zmq.Context()
    hb = ctx.socket(zmq.REQ)
    hb.connect(REQ_ADDR)

    while True:
        try:
            hb.send(msgpack.packb({
                "service": "heartbeat",
                "data": {"user": username}
            }, use_bin_type=True))
            hb.recv()
        except:
            pass

        time.sleep(5)

# ---------------------------------------------------
# Gera frases
# ---------------------------------------------------
def gerar_resposta(nome):
    frases = [
        "Alguém viu algum filme bom ultimamente?",
        "Tô procurando algo leve pra assistir hoje.",
        "Esse mês saiu muito filme bom.",
        "Vocês gostam mais de filme dublado ou legendado?",
        "Tô com vontade de rever um clássico.",
        "Preciso de uma recomendação urgente.",
        "Nada supera assistir filme comendo pipoca.",
        "Filme bom é aquele que faz pensar.",
        "Tem filme que devia ter continuação.",
        "Assisti um filme aleatório e adorei.",
        "Alguém já viu o novo filme do Dune? Vale a pena?",
"Preciso de uma comédia leve pra hoje, ideias?",
"Assisti ‘Interstellar’ de novo… nunca canso.",
"O final de ‘A Origem’ me deixa irritado até hoje.",
"Qual o melhor filme do Tarantino pra começar?",
"‘Clube da Luta’ é realmente tudo isso?",
"Revi ‘Shrek’ e continua perfeito.",
"Recomendam algum terror psicológico?",
"‘Parasita’ me destruiu emocionalmente.",
"Alguém entendeu Tenet ou só fingiu?",
"Top 3 filmes de ficção científica, vai!",
"As músicas da trilha de ‘La La Land’ são absurdas.",
"Aquele plot twist em ‘Ilha do Medo’… sem palavras.",
"Qual o melhor filme da Marvel na opinião de vocês?",
"Senti falta de mais ação no novo Batman.",
"Queria esquecer ‘O Senhor dos Anéis’ pra assistir de novo.",
"Hora do clássico: vou rever ‘Forrest Gump’.",
"Preciso de um filme triste pra chorar, sugestões?",
"O CGI de hoje estraga um pouco a magia às vezes.",
"Alguém viu o filme novo do Miyazaki?",
"A trilogia do Nolan do Batman ainda é insuperável.",
"Adoro filmes com viagem no tempo.",
"‘Coringa’ foi pesado, mas incrível.",
"Quais filmes vocês acham superestimados?",
"Assisti ‘Whiplash’ e ainda estou tenso.",
"Gosto muito de filmes baseados em fatos reais.",
"Filmes dos anos 80 têm outro charme.",
"‘Matrix’ revolucionou tudo.",
"Preciso de um romance leve, indicações?",
"‘Her’ é o filme mais bonito que já vi.",
"Assistir musical sozinho é estranho?",
"Filmes em preto e branco ainda me surpreendem.",
"Revi ‘O Rei Leão’ e chorei de novo.",
"Qual melhor filme pra ver com amigos?",
"Queria um filme de aventura estilo Indiana Jones.",
"A fotografia de ‘Blade Runner 2049’ é perfeita.",
"Não acredito que nunca vi ‘Pulp Fiction’, vou corrigir isso.",
"Filme de terror que realmente assusta?",
"A Pixar não sabe errar.",
"‘Oppenheimer’ me deixou pensativo demais.",
"Assistiram ‘Tudo em Todo Lugar ao Mesmo Tempo’? Maluco e genial.",
"A dublagem brasileira é muito boa, né?",
"Mais alguém ama filmes policiais?",
"Hoje vou ver um documentário, sugestões?",
"Preciso de um filme curto, menos de 90 min.",
"DiCaprio sempre entrega.",
"Gosto muito de filmes de tribunal.",
"O final de ‘Noivo Neurótico, Noiva Nervosa’ é ótimo.",
"Filme ruim que vocês amam mesmo assim?",
"As animações japonesas são outro nível.",
"Revi ‘Harry Potter’ e bateu nostalgia.",
"A trilha sonora de Hans Zimmer é sempre surreal.",
"Gosto de filmes que me deixam confuso.",
"Preciso de um suspense estilo ‘Seven’.",
"Filme com plot twist surpreendente?",
"Já viram o novo da A24?",
"‘Os Infiltrados’ é simplesmente perfeito.",
"Qual filme te fez rir até doer?",
"Hoje tô na vibe de drama.",
"Recomendações de filmes com viagem espacial?",
"Ainda não superei ‘Toy Story 3’.",
"Assistiram Barbie? É engraçado demais.",
"Quais filmes vocês acham que todo mundo deveria ver?",
"Maratona de animações hoje.",
"Sinto falta de filmes de fantasia estilo antigos.",
"Os filmes do Wes Anderson são lindos visualmente.",
"Qual o filme mais assustador que existe?",
"Assistir filme de madrugada tem outro clima.",
"Quero um filme bem inteligente, tipo mind-bending.",
"‘Green Book’ é tão confortante.",
"Existe algum filme perfeito pra vocês?",
"Vi ‘Oldboy’ e minha mente explodiu.",
"Filmes de zumbi recomendados?",
"Quero ver um clássico que nunca assisti.",
"Hoje vou rever ‘Gladiador’.",
"Drama romântico pra chorar muito?",
"Curto filmes independentes.",
"Qual o melhor filme de 2024 segundo vocês?",
"Quero ver algo com reviravoltas intensas.",
"‘Mad Max: Estrada da Fúria’ continua incrível.",
"Filme de espionagem estilo ‘Bourne’?",
"‘O Grande Gatsby’ é muito estiloso.",
"Filmes sobre multiverso tão ficando populares.",
"Gosto de filmes longos, 3 horas mesmo.",
"Comédia romântica pra ver com meu parceiro?",
"Qual o filme mais estranho que vocês amam?",
"Amo filmes com estética retrô.",
"Documentários de crime prendem muito.",
"Queria um filme de ação bem frenético.",
"‘Gravidade’ no cinema foi surreal.",
"Qual filme todo mundo gosta mas você não?",
"Hoje vou ver terror trash, por que não.",
"Filmes de época me relaxam.",
"‘Cisne Negro’ foi muito intenso.",
"Quero um filme sobre amizade.",
"Algo no estilo ‘O Jogo da Imitação’?",
"Mais alguém viciado em maratonar trilogias?",
"Filmes sobre robôs sempre me prendem.",
"Queria algo futurista hoje.",
"Fechando a noite com um clássico do Spielberg."
    ]
    return random.choice(frases)

# ---------------------------------------------------
# LISTENER — PUB/SUB é texto puro
# ---------------------------------------------------
def listen_messages(username, sub):
    while True:
        try:
            topic = sub.recv_string()
            msg   = sub.recv_string()

            # --------------------------------------------------
            # Formatos atuais:
            # Pública: [canal][user] mensagem (timestamp)
            # Privada: [PM][user] mensagem (timestamp)
            # --------------------------------------------------

            user = "???"
            texto = msg
            msg_formatada = msg

            try:
                if msg.startswith("[") and msg.count("]") >= 2:

                    # Fecha primeiro bloco
                    bloco1_end = msg.find("]")

                    # Começa segundo bloco
                    bloco2_start = msg.find("[", bloco1_end + 1)
                    bloco2_end   = msg.find("]", bloco2_start + 1)

                    # Extrai canal ou PM
                    bloco1 = msg[1:bloco1_end]

                    # Extrai usuário
                    user = msg[bloco2_start+1 : bloco2_end]

                    # Extrai texto sem timestamp
                    resto = msg[bloco2_end + 1 :].strip()
                    texto = resto.rsplit("(", 1)[0].strip()

                    msg_formatada = f"{user}: {texto}"

                else:
                    msg_formatada = msg

            except:
                msg_formatada = msg

            # PRIVADA
            if topic.lower() == username.lower():
                log_print(f"💌 PRIVADA → {msg_formatada}", username)
                continue

            # PÚBLICA
            log_print(f"[{topic}] {msg_formatada}", username)

        except Exception as e:
            log_print(f"❌ Erro SUB: {e}", username)
            time.sleep(1)

# ---------------------------------------------------
# BOT PRINCIPAL
# ---------------------------------------------------
def main():
    username = random_name()

    print("\n" + "="*60)
    print(f"🤖 BOT INICIADO: {username}")
    print("="*60 + "\n")

    ctx = zmq.Context()

    req = ctx.socket(zmq.REQ)
    req.connect(REQ_ADDR)

    sub = ctx.socket(zmq.SUB)
    sub.connect(SUB_ADDR)

    # LOGIN
    resp = send_request(req, "login", {"user": username, "timestamp": now_iso()})
    log_print(f"LOGIN: {resp['data']['status']}", username)

    # Heartbeat
    threading.Thread(target=send_heartbeat, args=(username,), daemon=True).start()

    # Carrega canais
    resp_channels = send_request(req, "channels", {})
    channels = resp_channels["data"]["channels"]

    if not channels:
        send_request(req, "channel", {"channel": "Geral"})
        channels = ["Geral"]

    salas = random.sample(channels, random.randint(1, min(3,len(channels))))
    canal_privado = username.lower()

    log_print(f"Canais inscritos: {salas}", username)
    log_print(f"Inbox privada: {canal_privado}", username)

    # SUBSCRIBE
    for ch in salas:
        sub.setsockopt_string(zmq.SUBSCRIBE, ch)
        send_request(req, "subscribe", {"user": username, "channel": ch})

    sub.setsockopt_string(zmq.SUBSCRIBE, canal_privado)
    send_request(req, "subscribe", {"user": username, "channel": canal_privado})

    threading.Thread(target=listen_messages, args=(username, sub), daemon=True).start()

    # LOOP PRINCIPAL
    while True:
        time.sleep(random.uniform(4, 8))

        modo = random.choice(["publica", "privada"])

        # Publica
        if modo == "publica":
            canal = random.choice(salas)
            texto = gerar_resposta(username)

            send_request(req, "publish", {
                "user": username,
                "channel": canal,
                "message": texto,
                "timestamp": now_iso()
            })

            log_print(f"[{canal}] {username}: {texto}", username)


        # Privada
        else:
            users_resp = send_request(req, "users", {})
            online = [u for u in users_resp["data"]["users"] if u.lower() != username.lower()]

            if not online:
                continue

            destino = random.choice(online)
            texto = gerar_resposta(username)

            resp_pm = send_request(req, "message", {
                "src": username,
                "dst": destino,
                "message": texto,
                "timestamp": now_iso()
            })

            status = resp_pm.get("data", {}).get("status", "").lower()

            if status in ["ok", "sucesso", "success"]:
                log_print(f"📤 {username} enviou mensagem PRIVADA para {destino}: {texto}", username)
                log_print(f"📨 {destino} recebeu mensagem PRIVADA de {username}: {texto}", username)
            else:
                log_print(f"❌ Falha ao enviar mensagem PRIVADA para {destino}", username)


if __name__ == "__main__":
    main()