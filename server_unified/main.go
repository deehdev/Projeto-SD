package main

import (
    "log"
    "os"
    "strconv"
    "time"

    zmq "github.com/pebbe/zmq4"
)

func main() {

    // =====================================================
    // Variáveis de ambiente
    // =====================================================
    serverName = os.Getenv("SERVER_NAME")
    if serverName == "" {
        serverName = "server"
        log.Printf("[MAIN][AVISO] SERVER_NAME não definido. Usando padrão: %s", serverName)
    }

    pstr := os.Getenv("SERVER_REP_PORT")
    if pstr == "" {
        repPort = 7000
        log.Printf("[MAIN][AVISO] SERVER_REP_PORT não definido. Usando padrão: %d", repPort)
    } else {
        v, _ := strconv.Atoi(pstr)
        repPort = v
        log.Printf("[MAIN][INFO] Porta REP configurada via env: %d", repPort)
    }

    refAddr = os.Getenv("REF_ADDR")
    if refAddr == "" {
        refAddr = "tcp://ref:6000"
        log.Printf("[MAIN][AVISO] REF_ADDR não definido. Usando padrão: %s", refAddr)
    }

    proxyPubAddr = os.Getenv("PROXY_PUB_ADDR")
    if proxyPubAddr == "" {
        proxyPubAddr = "tcp://proxy:5557"
        log.Printf("[MAIN][AVISO] PROXY_PUB_ADDR não definido. Usando padrão: %s", proxyPubAddr)
    }

    // =====================================================
    // Carregar persistência
    // =====================================================
    _ = loadJSON(usersFile, &users)
    _ = loadJSON(channelsFile, &channels)
    _ = loadJSON(subsFile, &subscriptions)
    _ = loadJSON(logsFile, &logs)

    if users == nil {
        users = []string{}
    }
    if channels == nil {
        channels = []string{}
    }
    if subscriptions == nil {
        subscriptions = map[string][]string{}
    }
    if logs == nil {
        logs = []LogEntry{}
    }

    // =====================================================
    // Criar contexto ZMQ
    // =====================================================
    ctx, err := zmq.NewContext()
    if err != nil {
        log.Fatalf("[MAIN][ERRO] Falha ao criar contexto ZMQ: %v", err)
    }
    defer ctx.Term()

    // =====================================================
    // Socket PUB (replicação / publish)
    // =====================================================
    pub, err := ctx.NewSocket(zmq.PUB)
    if err != nil {
        log.Fatalf("[MAIN][ERRO] Falha ao criar socket PUB: %v", err)
    }
    defer pub.Close()
    pub.SetLinger(0)

    if err := pub.Connect(proxyPubAddr); err != nil {
        log.Printf("[MAIN][AVISO] Falha ao conectar PUB ao proxy: %v", err)
    } else {
        log.Printf("[MAIN][INFO] PUB conectado ao proxy: %s", proxyPubAddr)
    }

    // Salvar socket global para uso no handlePublish()
    pubSocketMu.Lock()
    pubSocket = pub
    pubSocketMu.Unlock()

    // =====================================================
    // Socket REP (clientes → servidor)
    // =====================================================
    rep, err := ctx.NewSocket(zmq.REP)
    if err != nil {
        log.Fatalf("[MAIN][ERRO] Falha ao criar socket REP: %v", err)
    }
    defer rep.Close()
    rep.SetLinger(0)

    bind := "tcp://*:" + strconv.Itoa(repPort)
    if err := rep.Bind(bind); err != nil {
        log.Fatalf("[MAIN][ERRO] Falha ao bind REP em %s: %v", bind, err)
    }
    log.Printf("[MAIN][INFO] REP aguardando em %s", bind)

    // =====================================================
    // Socket SUB (replicação + eleição)
    // =====================================================
    sub, err := ctx.NewSocket(zmq.SUB)
    if err != nil {
        log.Fatalf("[MAIN][ERRO] Falha ao criar socket SUB: %v", err)
    }
    defer sub.Close()
    sub.SetLinger(0)

    if err := sub.Connect("tcp://proxy:5558"); err != nil {
        log.Printf("[MAIN][AVISO] Falha ao conectar SUB ao proxy: %v", err)
    } else {
        sub.SetSubscribe("replicate")
        sub.SetSubscribe("servers")
        log.Printf("[MAIN][INFO] SUB conectado ao proxy: tcp://proxy:5558")
    }

    // =====================================================
    // Iniciar loops REP e SUB
    // =====================================================
    log.Printf("[MAIN][INFO] Iniciando loops REP e SUB...")
    go repLoop(rep, pub)
    go subLoop(sub)

    // =====================================================
    // Heartbeat periódico
    // =====================================================
    go func() {
        for {
            time.Sleep(5 * time.Second)
            req := Envelope{
                Service:   "heartbeat",
                Data:      map[string]interface{}{"user": serverName, "port": repPort},
                Timestamp: nowISO(),
                Clock:     incClockBeforeSend(),
            }
            _, err := directReqZMQJSON(refAddr, req, 2*time.Second)
            if err != nil {
                log.Printf("[HEARTBEAT][ERRO] Falha ao enviar heartbeat ao REF: %v", err)
            }
        }
    }()

    // =====================================================
    // Registro inicial no REF
    // =====================================================
    rank := requestRank()
    log.Printf("[REF][INFO] Rank recebido: %d", rank)

    if lst, err := requestList(); err == nil {
        log.Printf("[REF][INFO] Lista de servidores: %v", lst)
    }

    // =====================================================
    // Determinar coordenador
    // =====================================================
    if coord, err := determineCoordinator(); err == nil {
        currentCoordinatorMu.Lock()
        currentCoordinator = coord
        currentCoordinatorMu.Unlock()
        log.Printf("[MAIN][INFO] Coordenador atual: %s", coord)
    }

    // =====================================================
    // Sincronização inicial
    // =====================================================
    log.Printf("[MAIN][INFO] Solicitando sincronização inicial...")
    requestInitialSync()

    // =====================================================
    // Loop infinito
    // =====================================================
    log.Printf("[MAIN][INFO] Servidor inicializado com sucesso! Aguardando eventos...")
    select {}
}
