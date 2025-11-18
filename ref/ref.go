// ref.go
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
	// opcional incremento pós-recebimento
	logicalClock++
	clockMu.Unlock()
}

func pruneLoop() {
	for {
		time.Sleep(5 * time.Second)
		now := time.Now().Unix()
		mu.Lock()
		for name, s := range servers {
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
		// fallback: enviar algo simples (não deve acontecer)
		log.Println("[REF] erro ao serializar resposta:", err)
		rep.SendMessage([]byte("ERR"))
		return
	}
	rep.SendBytes(out, 0)
}

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
			// não podemos continuar sem responder — enviar envelope de erro
			replyWithEnvelope(rep, "ref", map[string]interface{}{"error": "recv error"})
			continue
		}

		var req ReqEnv
		if err := msgpack.Unmarshal(raw, &req); err != nil {
			log.Println("[REF] msgpack decode err:", err)
			// responder sempre com envelope válido
			replyWithEnvelope(rep, "ref", map[string]interface{}{"error": "msgpack decode error"})
			continue
		}

		// atualiza relógio lógico do REF a partir do clock recebido (se houver)
		updateClock(req.Clock)

		service := req.Service
		name := ""
		if v := req.Data["user"]; v != nil {
			if s, ok := v.(string); ok {
				name = s
			}
		}
		addr := ""
		if v := req.Data["addr"]; v != nil {
			if s, ok := v.(string); ok {
				addr = s
			}
		}

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
				nextRank++
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
				srv = &ServerInfo{
					Name:     name,
					Rank:     nextRank,
					Addr:     addr,
					LastSeen: time.Now().Unix(),
				}
				servers[name] = srv
				log.Printf("[REF] Novo servidor via heartbeat: %s rank=%d addr=%s", name, nextRank, addr)
				nextRank++
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
