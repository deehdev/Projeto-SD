package main

import (
	"log"
	"sync"
	"time"

	zmq "github.com/pebbe/zmq4"
	"github.com/vmihailenco/msgpack/v5"
)

// ReqEnv representa o envelope de requisição que o REF recebe.
type ReqEnv struct {
	Service string                 `msgpack:"service"`
	Data    map[string]interface{} `msgpack:"data"`
	Clock   int                    `msgpack:"clock"`
}

// Envelope usado nas respostas do REF (compatível com os servidores).
type Envelope struct {
	Service   string                 `msgpack:"service"`
	Data      map[string]interface{} `msgpack:"data"`
	Timestamp string                 `msgpack:"timestamp"`
	Clock     int                    `msgpack:"clock"`
}

// ServerInfo guarda metadados dos servidores registrados no REF.
type ServerInfo struct {
	Name     string `msgpack:"name"`
	Rank     int    `msgpack:"rank"`
	Addr     string `msgpack:"addr"`
	LastSeen int64  `msgpack:"last_seen"`
}

var (
	mu           sync.Mutex
	servers      = map[string]*ServerInfo{}
	nextRank     = 1
	logicalClock = 0
	clockMu      sync.Mutex
)

// ----------------------- Funções Auxiliares -----------------------

func getString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return ""
}

func getInt(v interface{}) int {
	if i, ok := v.(int); ok {
		return i
	}
	if f, ok := v.(float64); ok {
		return int(f) 
	}
	if i, ok := v.(int64); ok {
		return int(i)
	}
	return 0
}

// ----------------------- Relógio e Utilidades -----------------------

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}

func incClock() int {
	clockMu.Lock()
	logicalClock++
	v := logicalClock
	clockMu.Unlock()
	return v
}

func updateClock(n int) {
	clockMu.Lock()
	if n > logicalClock {
		logicalClock = n
	}
	logicalClock++
	clockMu.Unlock()
}

func pruneLoop() {
	for {
		time.Sleep(5 * time.Second)
		now := time.Now().Unix()
		mu.Lock()
		for name, s := range servers {
			// Remove se não for visto há mais de 15 segundos
			if now-s.LastSeen > 15 {
				log.Printf("[REF] Removendo servidor inativo: %s", name)
				delete(servers, name)
			}
		}
		mu.Unlock()
	}
}

func replyWithEnvelope(rep *zmq.Socket, service string, data map[string]interface{}) {
	env := Envelope{
		Service:   service,
		Data:      data,
		Timestamp: nowISO(),
		Clock:     incClock(),
	}
	out, err := msgpack.Marshal(env)
	if err != nil {
		log.Println("[REF] erro ao serializar resposta:", err)
		rep.SendMessage([]byte("ERR"))
		return
	}
	rep.SendBytes(out, 0)
}

// ----------------------- MAIN -----------------------

func main() {
	log.Println("[REF] Iniciado em tcp://*:5550 (ZMQ REP)")
	go pruneLoop()

	ctx, _ := zmq.NewContext()
	defer ctx.Term()
	rep, _ := ctx.NewSocket(zmq.REP)
	defer rep.Close()
	if err := rep.Bind("tcp://*:5550"); err != nil {
		log.Fatalf("[REF] erro bind: %v", err)
	}

	for {
		raw, err := rep.RecvBytes(0)
		if err != nil {
			log.Println("[REF] recv err:", err)
			replyWithEnvelope(rep, "ref", map[string]interface{}{"error": "recv error"})
			continue
		}

		var req ReqEnv
		if err := msgpack.Unmarshal(raw, &req); err != nil {
			log.Println("[REF] msgpack decode err:", err)
			replyWithEnvelope(rep, "ref", map[string]interface{}{"error": "msgpack decode error"})
			continue
		}

		// Atualiza relógio lógico do REF a partir do clock recebido
		updateClock(req.Clock)

		service := req.Service
		// **Usando funções auxiliares para extrair dados**
		name := getString(req.Data["user"])
		addr := getString(req.Data["addr"])

		switch service {
		case "rank":
			if name == "" {
				replyWithEnvelope(rep, "ref", map[string]interface{}{"error": "missing user"})
				continue
			}
			mu.Lock()
			srv, exists := servers[name]
			if !exists {
				srv = &ServerInfo{
					Name:     name,
					Rank:     nextRank,
					Addr:     addr,
					LastSeen: time.Now().Unix(),
				}
				servers[name] = srv
				log.Printf("[REF] Novo servidor registrado: %s rank=%d addr=%s", name, nextRank, addr)
				nextRank++ // Incrementa SÓ se for novo
			} else {
				// atualiza lastseen e addr se mudou
				srv.LastSeen = time.Now().Unix()
				if addr != "" && addr != srv.Addr {
					log.Printf("[REF] Atualizou addr do servidor %s -> %s", name, addr)
					srv.Addr = addr
				}
			}
			resp := map[string]interface{}{"rank": srv.Rank}
			mu.Unlock()
			replyWithEnvelope(rep, "ref", resp)

		case "heartbeat":
			if name == "" {
				replyWithEnvelope(rep, "ref", map[string]interface{}{"error": "missing user"})
				continue
			}
			mu.Lock()
			srv, exists := servers[name]
			if !exists {
				// Servidor faz heartbeat mas nunca pediu rank (registra com um novo)
				srv = &ServerInfo{
					Name:     name,
					Rank:     nextRank,
					Addr:     addr,
					LastSeen: time.Now().Unix(),
				}
				servers[name] = srv
				log.Printf("[REF] Novo servidor via heartbeat: %s rank=%d addr=%s", name, nextRank, addr)
				nextRank++ // Incrementa SÓ se for novo
			} else {
				srv.LastSeen = time.Now().Unix()
				if addr != "" && addr != srv.Addr {
					log.Printf("[REF] Atualizou addr do servidor %s -> %s", name, addr)
					srv.Addr = addr
				}
			}
			mu.Unlock()
			replyWithEnvelope(rep, "ref", map[string]interface{}{"status": "ok"})

		case "list":
			mu.Lock()
			list := []map[string]interface{}{}
			for _, s := range servers {
				list = append(list, map[string]interface{}{
					"name": s.Name,
					"rank": s.Rank,
					"addr": s.Addr,
				})
			}
			mu.Unlock()
			replyWithEnvelope(rep, "ref", map[string]interface{}{"list": list})

		default:
			replyWithEnvelope(rep, "ref", map[string]interface{}{"error": "serviço desconhecido"})
		}
	}
}