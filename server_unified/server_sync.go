package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	zmq "github.com/pebbe/zmq4"
)

//
// ========================================================
// REP LOOP – processa requisições dos clientes
// ========================================================
func repLoop(rep *zmq.Socket, pub *zmq.Socket) {
	for {
		raw, err := rep.RecvBytes(0)
		if err != nil {
			log.Println("[REP][ERRO]", err)
			continue
		}

		// tentar msgpack; fallback JSON
		env := msgpackUnmarshal(raw)
		if env.Service == "" {
			env = jsonUnmarshal(raw)
		}

		updateClockOnRecv(env.Clock)

		var resp Envelope

		switch env.Service {
		case "login":
			resp = handleLogin(env)
		case "channels":
			resp = handleListChannels(env)
		case "channel":
			resp = handleCreateChannel(env)
		case "publish":
			resp = handlePublish(env)
		case "message":
			resp = handleMessage(env)
		case "subscribe":
			resp = handleSubscribe(env)
		case "unsubscribe":
			resp = handleUnsubscribe(env)
		case "sync_request":
			resp = handleSyncRequest(env)
		case "heartbeat":
			resp = Envelope{
				Service:   "ok",
				Timestamp: nowISO(),
				Clock:     incClockBeforeSend(),
			}
		default:
			resp = Envelope{
				Service:   "erro",
				Timestamp: nowISO(),
				Clock:     incClockBeforeSend(),
				Data: map[string]interface{}{
					"message": "serviço desconhecido",
				},
			}
		}

		// enviar resposta
		out := msgpackMarshal(resp)
		if len(out) > 0 {
			rep.SendBytes(out, 0)
		} else {
			j, _ := json.Marshal(resp)
			rep.SendBytes(j, 0)
		}
	}
}

//
// ========================================================
// SUB LOOP – replicação + eleição
// ========================================================
func subLoop(sub *zmq.Socket) {
	for {
		parts, err := sub.RecvMessageBytes(0)
		if err != nil {
			log.Println("[SUB][ERRO]", err)
			continue
		}

		if len(parts) < 2 {
			continue
		}

		topic := string(parts[0])
		payload := parts[1]

		env := msgpackUnmarshal(payload)
		if env.Service == "" {
			env = jsonUnmarshal(payload)
		}

		updateClockOnRecv(env.Clock)

		switch topic {
		case "replicate":
			applyReplication(env)
		case "servers":
			handleCoordinatorChange(env)
		}
	}
}

//
// ========================================================
// HANDLE SYNC REQUEST (coordenador → nó novo)
// ========================================================
func handleSyncRequest(env Envelope) Envelope {
	logsMu.Lock()
	cp := make([]map[string]interface{}, 0, len(logs))

	for _, le := range logs {
		cp = append(cp, map[string]interface{}{
			"id":        le.ID,
			"type":      le.Type,
			"data":      le.Data,
			"timestamp": le.Timestamp,
			"clock":     le.Clock,
		})
	}

	logsMu.Unlock()

	return Envelope{
		Service:   "sync_response",
		Timestamp: nowISO(),
		Clock:     incClockBeforeSend(),
		Data:      map[string]interface{}{"logs": cp},
	}
}

//
// ========================================================
// APPLY SYNC RESPONSE (nó novo aplica logs)
// ========================================================
func applySyncResponse(env Envelope) {
	raw, ok := env.Data["logs"]
	if !ok {
		log.Println("[SYNC][ERRO] sync_response sem logs")
		return
	}

	list, ok := raw.([]interface{})
	if !ok {
		log.Println("[SYNC][ERRO] formato inválido")
		return
	}

	newLogs := []LogEntry{}

	for _, it := range list {
		m := it.(map[string]interface{})

		clock := 0
		switch v := m["clock"].(type) {
		case float64:
			clock = int(v)
		case int:
			clock = v
		}

		newLogs = append(newLogs, LogEntry{
			ID:        fmt.Sprint(m["id"]),
			Type:      fmt.Sprint(m["type"]),
			Timestamp: fmt.Sprint(m["timestamp"]),
			Data:      m["data"].(map[string]interface{}),
			Clock:     clock,
		})
	}

	logsMu.Lock()
	logs = newLogs
	logsMu.Unlock()

	persistLogs()

	log.Printf("[SYNC] Sincronizados %d logs.", len(newLogs))
}

//
// ========================================================
// DETERMINE COORDINATOR – menor rank
// ========================================================
func determineCoordinator() (string, error) {

	list, err := requestList()
	if err != nil {
		return "", err
	}

	if len(list) == 0 {
		return serverName, nil
	}

	type node struct {
		Name string
		Rank int
	}

	nodes := []node{}

	for _, item := range list {
		nodes = append(nodes, node{
			Name: item["name"].(string),
			Rank: int(item["rank"].(float64)),
		})
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Rank < nodes[j].Rank
	})

	// prefixo correto do docker compose
	coord := nodes[0].Name
	return "server_" + coord, nil
}

//
// ========================================================
// handleCoordinatorChange – recebido via PUB
// ========================================================
func handleCoordinatorChange(env Envelope) {
	coord := fmt.Sprintf("%v", env.Data["coordinator"])

	currentCoordinatorMu.Lock()
	currentCoordinator = coord
	currentCoordinatorMu.Unlock()

	log.Printf("[COORD] Novo coordenador: %s", coord)
}

//
// ========================================================
// requestInitialSync – nó novo busca logs
// ========================================================
func requestInitialSync() {

	currentCoordinatorMu.Lock()
	coord := currentCoordinator // já vem como "server_a"
	currentCoordinatorMu.Unlock()

	if coord == "" {
		log.Println("[SYNC] Nenhum coordenador definido")
		return
	}

	list, err := requestList()
	if err != nil {
		log.Println("[SYNC] Erro requestList:", err)
		return
	}

	port := 0
	for _, s := range list {
		// nomes do REF: "a", "b", "c"
		// precisar virar "server_a", "server_b" etc.
		nameShort := s["name"].(string)
		fullName := "server_" + nameShort

		if fullName == coord {
			port = int(s["port"].(float64))
		}
	}

	if port == 0 {
		log.Println("[SYNC] Porta do coordenador não encontrada")
		return
	}

	addr := fmt.Sprintf("tcp://%s:%d", coord, port)
	log.Println("[SYNC] Solicitando logs para", addr)

	req := Envelope{
		Service:   "sync_request",
		Timestamp: nowISO(),
		Clock:     incClockBeforeSend(),
		Data:      map[string]interface{}{},
	}

	rep, err := directReqZMQ(addr, req, 5*time.Second)
	if err != nil {
		log.Println("[SYNC] Falhou:", err)
		return
	}

	applySyncResponse(rep)
}

//
// ========================================================
// requestRank / requestList – comunicação com REF
// ========================================================
func requestRank() int {
	req := Envelope{
		Service: "rank",
		Data: map[string]interface{}{
			"user": serverName,
			"port": repPort,
		},
		Timestamp: nowISO(),
		Clock:     incClockBeforeSend(),
	}

	rep, err := directReqZMQJSON(refAddr, req, 4*time.Second)
	if err != nil {
		log.Printf("[REF][ERRO] Falha pedir rank: %v", err)
		return 0
	}

	if f, ok := rep.Data["rank"].(float64); ok {
		return int(f)
	}
	return 0
}

func requestList() ([]map[string]interface{}, error) {
	req := Envelope{
		Service:   "list",
		Data:      map[string]interface{}{},
		Timestamp: nowISO(),
		Clock:     incClockBeforeSend(),
	}

	rep, err := directReqZMQJSON(refAddr, req, 4*time.Second)
	if err != nil {
		return nil, err
	}

	raw := rep.Data["list"].([]interface{})
	out := []map[string]interface{}{}

	for _, e := range raw {
		out = append(out, e.(map[string]interface{}))
	}

	return out, nil
}

//
// ========================================================
// Fallback JSON
// ========================================================
func jsonUnmarshal(b []byte) Envelope {
	var env Envelope
	if json.Unmarshal(b, &env) == nil {
		return env
	}
	return Envelope{}
}
