package main

import (
	"encoding/json"
	"log"
	"net"
	"sync"
	"time"
)

type Envelope struct {
	Service   string                 `json:"service"`
	Data      map[string]interface{} `json:"data"`
	Timestamp string                 `json:"timestamp"`
	Clock     int                    `json:"clock"`
}

type ServerInfo struct {
	Name     string `json:"name"`
	Rank     int    `json:"rank"`
	Port     int    `json:"port"`
	LastSeen int64  `json:"last_seen"`
}

var (
	mu           sync.Mutex
	servers      = map[string]*ServerInfo{}
	nextRank     = 1
	logicalClock = 0
)

func now() string {
	return time.Now().Format(time.RFC3339Nano)
}

func incClock() int {
	mu.Lock()
	logicalClock++
	v := logicalClock
	mu.Unlock()
	return v
}

func updateClock(n int) {
	mu.Lock()
	if n > logicalClock {
		logicalClock = n
	}
	logicalClock++
	mu.Unlock()
}

func pruneLoop() {
	for {
		time.Sleep(5 * time.Second)
		mu.Lock()
		now := time.Now().Unix()
		for name, s := range servers {
			if now-s.LastSeen > 15 {
				log.Println("[REF] Removendo servidor inativo:", name)
				delete(servers, name)
			}
		}
		mu.Unlock()
	}
}

// ------------------------------
// SafePort: converte interface{} -> int SEM panic
// ------------------------------
func SafePort(v interface{}) int {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	default:
		return 0
	}
}

func handle(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 16*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	var req Envelope
	if err := json.Unmarshal(buf[:n], &req); err != nil {
		log.Println("[REF] JSON inválido:", err)
		return
	}

	updateClock(req.Clock)

	resp := Envelope{
		Service:   req.Service,
		Data:      map[string]interface{}{},
		Timestamp: now(),
		Clock:     incClock(),
	}

	switch req.Service {

	// ----------------------
	// RANK REQUEST
	// ----------------------
	case "rank":
		user, _ := req.Data["user"].(string)
		port := SafePort(req.Data["port"])

		mu.Lock()
		s, exists := servers[user]
		if !exists {
			s = &ServerInfo{
				Name:     user,
				Rank:     nextRank,
				Port:     port,
				LastSeen: time.Now().Unix(),
			}
			servers[user] = s
			log.Println("[REF] Novo servidor:", user, "rank:", nextRank, "port:", port)
			nextRank++
		} else {
			s.Port = port
			s.LastSeen = time.Now().Unix()
		}
		resp.Data["rank"] = s.Rank
		mu.Unlock()

	// ----------------------
	// HEARTBEAT
	// ----------------------
	case "heartbeat":
		user, _ := req.Data["user"].(string)
		port := SafePort(req.Data["port"])

		mu.Lock()
		if s, ok := servers[user]; ok {
			s.LastSeen = time.Now().Unix()
			if port != 0 {
				s.Port = port
			}
		}
		mu.Unlock()

		resp.Data["status"] = "ok"

	// ----------------------
	// LIST
	// ----------------------
	case "list":
		mu.Lock()
		list := []map[string]interface{}{}
		for _, s := range servers {
			list = append(list, map[string]interface{}{
				"name": s.Name,
				"rank": s.Rank,
				"port": s.Port,
			})
		}
		mu.Unlock()
		resp.Data["list"] = list
	}

	out, _ := json.Marshal(resp)
	conn.Write(out)
}

func main() {
	log.Println("[REF] Iniciado em TCP JSON :6000")

	go pruneLoop()

	ln, err := net.Listen("tcp", ":6000")
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handle(conn)
	}
}
