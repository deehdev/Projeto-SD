package main

import (
    "encoding/json"
    "log"
    "math"
    "sort"
    "strconv"
    "time"
)


// ----------------------------------------------
// JSON fallback
// ----------------------------------------------
func jsonUnmarshal(b []byte) (Envelope, error) {
    var env Envelope
    err := json.Unmarshal(b, &env)
    return env, err
}

// ----------------------------------------------
// requestRank
// ----------------------------------------------
func requestRank() int {
    req := Envelope{
        Service:   "rank",
        Timestamp: nowISO(),
        Clock:     incClockBeforeSend(),
        Data: map[string]interface{}{
            "user": serverName,
            "port": repPort,
        },
    }

    rep, err := directReqZMQJSON(refAddr, req, 4*time.Second)
    if err != nil {
        log.Printf("[REF][ERRO] Rank: %v", err)
        return 0
    }

    switch v := rep.Data["rank"].(type) {
    case float64:
        return int(v)
    case int:
        return v
    }
    return 0
}

// ----------------------------------------------
// requestList
// ----------------------------------------------
func requestList() ([]map[string]interface{}, error) {
    req := Envelope{
        Service:   "list",
        Timestamp: nowISO(),
        Clock:     incClockBeforeSend(),
        Data:      map[string]interface{}{},
    }

    rep, err := directReqZMQJSON(refAddr, req, 4*time.Second)
    if err != nil {
        return nil, err
    }

    // pode ser []interface{}
    if arr, ok := rep.Data["list"].([]interface{}); ok {
        out := []map[string]interface{}{}
        for _, item := range arr {
            out = append(out, item.(map[string]interface{}))
        }
        return out, nil
    }

    // ou já pode ser []map...
    if arr, ok := rep.Data["list"].([]map[string]interface{}); ok {
        return arr, nil
    }

    return nil, nil
}

// ----------------------------------------------
// applyCoordinatorUpdate
// ----------------------------------------------
func applyCoordinatorUpdate(env Envelope) {
    coord, ok := env.Data["coordinator"].(string)
    if !ok {
        log.Println("[COORD] Erro: campo 'coordinator' inválido")
        return
    }

    currentCoordinatorMu.Lock()
    currentCoordinator = coord
    currentCoordinatorMu.Unlock()

    log.Println("[COORD] Novo coordenador eleito:", coord)

    // ----------------------------------------------------------
    // 🔥 CHAVE DO PROJETO:
    // Assim que trocar de coordenador, todos servidores
    // devem sincronizar os logs automaticamente.
    // ----------------------------------------------------------
    go func() {
        // pequena espera para evitar corrida logo após eleição
        time.Sleep(800 * time.Millisecond)
        log.Println("[SYNC] Coordenador mudou — iniciando sincronização automática...")
        requestInitialSync()
    }()
}

// ----------------------------------------------
// determineCoordinator
// ----------------------------------------------
func determineCoordinator() (string, error) {
    list, err := requestList()
    if err != nil {
        return "", err
    }

    if len(list) == 0 {
        return "server_" + serverName, nil
    }

    type node struct {
        Name string
        Rank int
    }

    nodes := []node{}
    for _, s := range list {
        name := ""
        if v, ok := s["name"].(string); ok {
            name = v
        }

        rank := math.MaxInt32
        switch r := s["rank"].(type) {
        case float64:
            rank = int(r)
        case int:
            rank = r
        }

        nodes = append(nodes, node{Name: name, Rank: rank})
    }

    sort.Slice(nodes, func(i, j int) bool { return nodes[i].Rank < nodes[j].Rank })
    return "server_" + nodes[0].Name, nil
}

// ----------------------------------------------
// requestInitialSync
// ----------------------------------------------
func requestInitialSync() {
    currentCoordinatorMu.Lock()
    coord := currentCoordinator
    currentCoordinatorMu.Unlock()

    if coord == "" {
        log.Println("[SYNC] Sem coordenador.")
        return
    }

    list, err := requestList()
    if err != nil {
        log.Println("[SYNC] Erro requestList:", err)
        return
    }

    port := 0
    for _, s := range list {
        name := ""
        if v, ok := s["name"].(string); ok {
            name = v
        }

        if "server_"+name == coord {
            switch p := s["port"].(type) {
            case float64:
                port = int(p)
            case int:
                port = p
            }
        }
    }

    if port == 0 {
        log.Println("[SYNC] Porta do coordenador não encontrada")
        return
    }

    addr := "tcp://" + coord + ":" + strconv.Itoa(port)
    req := Envelope{
        Service:   "sync_request",
        Timestamp: nowISO(),
        Clock:     incClockBeforeSend(),
        Data:      map[string]interface{}{},
    }

    rep, err := directReqZMQ(addr, req, 4*time.Second)
    if err != nil {
        log.Println("[SYNC] Falhou:", err)
        return
    }

    logsRemote, ok := rep.Data["logs"].([]LogEntry)
    if ok {
        applySyncResponse(logsRemote)
    }
}
