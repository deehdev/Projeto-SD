// server_sync.go — Parte 4 (corrigido e mais robusto)
//
// Servidor sincronizado:
// - REF: JSON over TCP (rank/list/heartbeat)
// - Inter-server: ZMQ REQ/REP MessagePack
// - PUB/SUB (proxy): ZMQ PUB MessagePack payload
// - Lamport logical clock + Berkeley + Election

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	zmq "github.com/pebbe/zmq4"
	"github.com/vmihailenco/msgpack/v5"
)

/* =======================================================================
   Envelope (igual ao REF)
   ======================================================================= */

type Envelope struct {
	Service   string                 `json:"service" msgpack:"service"`
	Data      map[string]interface{} `json:"data" msgpack:"data"`
	Timestamp string                 `json:"timestamp" msgpack:"timestamp"`
	Clock     int                    `json:"clock" msgpack:"clock"`
}

/* =======================================================================
   Variáveis globais
   ======================================================================= */

var (
	mu           sync.Mutex
	logicalClock int

	name      string
	rank      int = -1
	refAddr   string
	proxyAddr string
	repPort   int

	// Berkeley
	clockOffset   int64
	msgCount      int
	berkeleyMutex sync.Mutex

	// Coordinator
	currentCoordinator   string
	currentCoordinatorMu sync.Mutex

	// minimal fix: serialize REQ calls to avoid REQ/REP deadlocks
	reqLock sync.Mutex
)

/* =======================================================================
   Clock helpers
   ======================================================================= */

func now() string {
	return time.Now().Format(time.RFC3339Nano)
}

func nowPhysical() int64 {
	return time.Now().Unix() + clockOffset
}

func incClockBeforeSend() int {
	mu.Lock()
	logicalClock++
	v := logicalClock
	mu.Unlock()
	return v
}

func updateClockRecv(n int) {
	mu.Lock()
	if n > logicalClock {
		logicalClock = n
	}
	logicalClock++
	mu.Unlock()
}

/* =======================================================================
   REF JSON helpers
   ======================================================================= */

// directReqJSON: envia json para addr (host:port) e retorna Envelope
func directReqJSON(addr string, env Envelope, timeout time.Duration) (Envelope, error) {
	var empty Envelope

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return empty, err
	}
	defer conn.Close()

	b, err := json.Marshal(env)
	if err != nil {
		return empty, err
	}

	// write
	if _, err := conn.Write(b); err != nil {
		return empty, err
	}

	// read (simples)
	buf := make([]byte, 32*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return empty, err
	}

	var rep Envelope
	if err := json.Unmarshal(buf[:n], &rep); err != nil {
		return empty, err
	}
	return rep, nil
}

/* =======================================================================
   ZMQ MessagePack helpers
   ======================================================================= */

// directReqZMQ: envia Envelope via REQ/REP MessagePack para addr (tcp://host:port)
// NOTE: usa reqLock para serializar chamadas REQ/REP (correção mínima)
func directReqZMQ(addr string, env Envelope, timeout time.Duration) (Envelope, error) {
	reqLock.Lock()
	defer reqLock.Unlock()

	var empty Envelope

	ctx, err := zmq.NewContext()
	if err != nil {
		return empty, err
	}
	defer ctx.Term()

	sock, err := ctx.NewSocket(zmq.REQ)
	if err != nil {
		return empty, err
	}
	defer sock.Close()

	sock.SetLinger(0)
	if err := sock.Connect(addr); err != nil {
		return empty, err
	}
	if timeout > 0 {
		_ = sock.SetRcvtimeo(timeout)
	}

	out, err := msgpack.Marshal(env)
	if err != nil {
		return empty, err
	}

	if _, err := sock.SendBytes(out, 0); err != nil {
		return empty, err
	}

	repBytes, err := sock.RecvBytes(0)
	if err != nil {
		return empty, err
	}

	var rep Envelope
	if err := msgpack.Unmarshal(repBytes, &rep); err != nil {
		return empty, err
	}
	return rep, nil
}

/* =======================================================================
   REF: rank / list / heartbeat
   ======================================================================= */

func requestRank() {
	req := Envelope{
		Service: "rank",
		Data: map[string]interface{}{
			"user": name,
			"port": repPort, // porta local para outros servidores usarem
		},
		Timestamp: now(),
		Clock:     incClockBeforeSend(),
	}

	rep, err := directReqJSON(refAddr, req, 5*time.Second)
	if err != nil {
		log.Println("[REF] rank erro:", err)
		return
	}

	updateClockRecv(rep.Clock)

	if v, ok := rep.Data["rank"]; ok {
		switch t := v.(type) {
		case float64:
			rank = int(t)
		case int:
			rank = t
		case int64:
			rank = int(t)
		default:
			// ignore unexpected
		}
	}

	log.Println("[REF] rank:", rank)
}

func requestList() ([]map[string]interface{}, error) {
	req := Envelope{
		Service:   "list",
		Data:      map[string]interface{}{},
		Timestamp: now(),
		Clock:     incClockBeforeSend(),
	}

	rep, err := directReqJSON(refAddr, req, 5*time.Second)
	if err != nil {
		return nil, err
	}

	updateClockRecv(rep.Clock)

	raw, ok := rep.Data["list"]
	if !ok || raw == nil {
		return nil, nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil, nil
	}

	out := make([]map[string]interface{}, 0, len(arr))
	for _, x := range arr {
		if m, ok := x.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

func sendHeartbeatLoop() {
	for {
		time.Sleep(5 * time.Second)
		req := Envelope{
			Service: "heartbeat",
			Data: map[string]interface{}{
				"user": name,
				"port": repPort,
			},
			Timestamp: now(),
			Clock:     incClockBeforeSend(),
		}
		if _, err := directReqJSON(refAddr, req, 5*time.Second); err != nil {
			log.Println("[REF] heartbeat erro:", err)
		}
	}
}

/* =======================================================================
   REP Listener (ZMQ)
   ======================================================================= */

func repLoop(rep *zmq.Socket) {
	for {
		b, err := rep.RecvBytes(0)
		if err != nil {
			log.Println("[REP] recv erro:", err)
			continue
		}

		var req Envelope
		if err := msgpack.Unmarshal(b, &req); err != nil {
			log.Println("[REP] msgpack decode erro:", err)
			continue
		}

		updateClockRecv(req.Clock)

		var resp Envelope

		switch req.Service {

		case "clock":
			resp = Envelope{
				Service: "clock",
				Data: map[string]interface{}{
					"time": nowPhysical(),
				},
				Timestamp: now(),
				Clock:     incClockBeforeSend(),
			}

		case "adjust":
			// conversão segura do campo "adjust"
			var adj int64 = 0
			if v, ok := req.Data["adjust"]; ok {
				switch t := v.(type) {
				case float64:
					adj = int64(t)
				case int:
					adj = int64(t)
				case int64:
					adj = t
				default:
					adj = 0
				}
			}

			berkeleyMutex.Lock()
			clockOffset += adj
			berkeleyMutex.Unlock()

			resp = Envelope{
				Service: "adjust",
				Data: map[string]interface{}{
					"applied": adj,
					"new":     nowPhysical(),
				},
				Timestamp: now(),
				Clock:     incClockBeforeSend(),
			}

		case "election":
			resp = Envelope{
				Service: "election",
				Data: map[string]interface{}{
					"election": "OK",
				},
				Timestamp: now(),
				Clock:     incClockBeforeSend(),
			}

		default:
			resp = Envelope{
				Service:   "error",
				Data:      map[string]interface{}{"msg": "unknown"},
				Timestamp: now(),
				Clock:     incClockBeforeSend(),
			}
		}

		out, err := msgpack.Marshal(resp)
		if err != nil {
			log.Println("[REP] msgpack marshal erro:", err)
			continue
		}
		if _, err := rep.SendBytes(out, 0); err != nil {
			log.Println("[REP] send erro:", err)
		}
	}
}

/* =======================================================================
   SUB Listener (coordenador updates)
   ======================================================================= */

func serversSubLoop(sub *zmq.Socket) {
	for {
		msgParts, err := sub.RecvMessageBytes(0)
		if err != nil {
			log.Println("[SUB servers] recv erro:", err)
			continue
		}
		if len(msgParts) < 2 {
			continue
		}

		var env Envelope
		if err := msgpack.Unmarshal(msgParts[1], &env); err != nil {
			log.Println("[SUB servers] msgpack decode erro:", err)
			continue
		}

		updateClockRecv(env.Clock)

		if env.Service == "election" {
			// aceita "coordenador" ou "coordinator" conforme quem publicou
			if c, ok := env.Data["coordenador"].(string); ok {
				currentCoordinatorMu.Lock()
				currentCoordinator = c
				currentCoordinatorMu.Unlock()
				log.Println("[SUB] novo coordenador:", c)
				continue
			}
			if c, ok := env.Data["coordinator"].(string); ok {
				currentCoordinatorMu.Lock()
				currentCoordinator = c
				currentCoordinatorMu.Unlock()
				log.Println("[SUB] novo coordinator:", c)
			}
		}
	}
}

/* =======================================================================
   Election + Berkeley + Publish
   ======================================================================= */

func publishCoordinator(pub *zmq.Socket, c string) {
	env := Envelope{
		Service: "election",
		Data: map[string]interface{}{
			"coordenador": c,
			"coordinator": c,
		},
		Timestamp: now(),
		Clock:     incClockBeforeSend(),
	}
	out, err := msgpack.Marshal(env)
	if err != nil {
		log.Println("[PUB] marshal erro:", err)
		return
	}
	// envia multipart: tópico + payload
	if _, err := pub.SendMessage("servers", out); err != nil {
		log.Println("[PUB] send erro:", err)
	}
}

func runBerkeleyCoordinator(pub *zmq.Socket) {
	log.Println("[BERKELEY] iniciando cálculo")

	list, err := requestList()
	if err != nil {
		log.Println("[BERKELEY] erro obtendo lista do REF:", err)
		return
	}
	if len(list) == 0 {
		log.Println("[BERKELEY] lista vazia — abortando")
		return
	}

	times := make(map[string]int64)
	times[name] = nowPhysical()

	for _, srv := range list {
		nRaw, ok := srv["name"]
		if !ok {
			continue
		}
		n, ok := nRaw.(string)
		if !ok || n == name {
			continue
		}

		// pega porta do REF (com conversão segura)
		var port int
		if pRaw, ok := srv["port"]; ok {
			switch pt := pRaw.(type) {
			case float64:
				port = int(pt)
			case int:
				port = pt
			case int64:
				port = int(pt)
			default:
				port = 0
			}
		}
		if port == 0 {
			log.Printf("[BERKELEY] porta inválida no REF para %s — pulando\n", n)
			continue
		}

		addr := fmt.Sprintf("tcp://server_%s:%d", n, port)
		req := Envelope{
			Service:   "clock",
			Data:      map[string]interface{}{},
			Timestamp: now(),
			Clock:     incClockBeforeSend(),
		}

		rep, err := directReqZMQ(addr, req, 3*time.Second)
		if err != nil {
			log.Printf("[BERKELEY] não conseguiu falar com %s (%s): %v\n", n, addr, err)
			continue
		}

		if tval, ok := rep.Data["time"]; ok {
			switch tv := tval.(type) {
			case float64:
				times[n] = int64(tv)
			case int:
				times[n] = int64(tv)
			case int64:
				times[n] = tv
			default:
				// ignore
			}
		}
	}

	if len(times) == 0 {
		log.Println("[BERKELEY] nenhuma amostra coletada")
		return
	}

	var sum int64
	for _, v := range times {
		sum += v
	}
	avg := int64(math.Round(float64(sum) / float64(len(times))))
	log.Printf("[BERKELEY] média calculada (epoch secs) = %d (samples=%d)\n", avg, len(times))

	for srvName, reported := range times {
		adjust := avg - reported

		if srvName == name {
			berkeleyMutex.Lock()
			clockOffset += adjust
			berkeleyMutex.Unlock()
			log.Printf("[BERKELEY] ajuste aplicado localmente: %d sec\n", adjust)
			continue
		}

		// resolve porta no list
		var port int
		for _, s := range list {
			if nm, ok := s["name"].(string); ok && nm == srvName {
				if pRaw, ok := s["port"]; ok {
					switch pt := pRaw.(type) {
					case float64:
						port = int(pt)
					case int:
						port = pt
					case int64:
						port = int(pt)
					}
				}
				break
			}
		}
		if port == 0 {
			log.Printf("[BERKELEY] porta desconhecida para %s — pulando ajuste\n", srvName)
			continue
		}

		addr := fmt.Sprintf("tcp://server_%s:%d", srvName, port)
		req := Envelope{
			Service: "adjust",
			Data: map[string]interface{}{
				"adjust": adjust,
			},
			Timestamp: now(),
			Clock:     incClockBeforeSend(),
		}
		if _, err := directReqZMQ(addr, req, 3*time.Second); err != nil {
			log.Printf("[BERKELEY] falha ao enviar ajuste para %s: %v\n", srvName, err)
		} else {
			log.Printf("[BERKELEY] ajuste enviado para %s: %d sec\n", srvName, adjust)
		}
	}
}

func maybeTriggerBerkeley(pub *zmq.Socket) {
	currentCoordinatorMu.Lock()
	c := currentCoordinator
	currentCoordinatorMu.Unlock()

	if c == "" {
		cand, err := determineCoordinator()
		if err != nil {
			log.Println("[BERKELEY] erro ao determinar coordenador:", err)
			return
		}
		c = cand
	}

	if c != name {
		log.Println("[BERKELEY] não sou coordenador:", c)
		return
	}

	runBerkeleyCoordinator(pub)
	publishCoordinator(pub, name)
}

func determineCoordinator() (string, error) {
	list, err := requestList()
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", fmt.Errorf("lista vazia")
	}

	best := ""
	bestRank := int64(1<<60 - 1)

	for _, s := range list {
		var rint int64 = 1<<60 - 1
		if rRaw, ok := s["rank"]; ok {
			switch rr := rRaw.(type) {
			case float64:
				rint = int64(rr)
			case int:
				rint = int64(rr)
			case int64:
				rint = rr
			}
		}
		n, _ := s["name"].(string)
		if n == "" {
			continue
		}
		if rint < bestRank {
			bestRank = rint
			best = n
		}
	}
	if best == "" {
		return "", fmt.Errorf("nenhum coordinator encontrado")
	}
	return best, nil
}

func publishLoop(pub *zmq.Socket) {
	i := 0
	for {
		time.Sleep(3 * time.Second)

		env := Envelope{
			Service: "sync",
			Data: map[string]interface{}{
				"msg": fmt.Sprintf("msg %d from %s", i, name),
			},
			Timestamp: now(),
			Clock:     incClockBeforeSend(),
		}

		out, err := msgpack.Marshal(env)
		if err != nil {
			log.Println("[PUB] marshal erro:", err)
			continue
		}

		// envia sem tópico (proxy/tratamento dependem do proxy)
		if _, err := pub.SendBytes(out, 0); err != nil {
			log.Println("[PUB] send erro:", err)
		}

		msgCount++
		i++

		if msgCount%10 == 0 {
			go maybeTriggerBerkeley(pub)
		}
	}
}

/* =======================================================================
   MAIN
   ======================================================================= */

func main() {
	name = os.Getenv("SERVER_NAME")
	if name == "" {
		name = "server_sync"
	}

	// porta padrão = 7000 se variável vazia / inválida
	pstr := os.Getenv("SERVER_REP_PORT")
	if pstr == "" {
		repPort = 7000
	} else {
		if v, err := strconv.Atoi(pstr); err == nil {
			repPort = v
		} else {
			repPort = 7000
		}
	}

	refAddr = os.Getenv("REF_ADDR")
	if refAddr == "" {
		refAddr = "ref:6000" // host:port (sem tcp://)
	}

	proxyAddr = os.Getenv("PROXY_PUB_ADDR")
	if proxyAddr == "" {
		proxyAddr = "tcp://proxy:5560"
	}

	log.Printf("[SYNC] start name=%s port=%d ref=%s proxy=%s\n", name, repPort, refAddr, proxyAddr)

	// ZMQ context
	ctx, err := zmq.NewContext()
	if err != nil {
		log.Fatalf("[MAIN] erro criando contexto ZMQ: %v", err)
	}
	defer ctx.Term()

	// PUB
	pub, err := ctx.NewSocket(zmq.PUB)
	if err != nil {
		log.Fatalf("[MAIN] erro criando PUB socket: %v", err)
	}
	defer pub.Close()
	pub.SetLinger(0)
	if err := pub.Connect(proxyAddr); err != nil {
		log.Println("[MAIN] warning: conectar PUB ao proxy falhou:", err)
	} else {
		log.Println("[MAIN] PUB conectado a", proxyAddr)
	}

	// REP
	rep, err := ctx.NewSocket(zmq.REP)
	if err != nil {
		log.Fatalf("[MAIN] erro criando REP socket: %v", err)
	}
	defer rep.Close()
	rep.SetLinger(0)
	bind := fmt.Sprintf("tcp://*:%d", repPort)
	if err := rep.Bind(bind); err != nil {
		log.Fatalf("[MAIN] erro bind REP %s: %v", bind, err)
	}
	log.Println("[MAIN] REP bind em", bind)

	// SUB para "servers"
	sub, err := ctx.NewSocket(zmq.SUB)
	if err != nil {
		log.Fatalf("[MAIN] erro criando SUB socket: %v", err)
	}
	defer sub.Close()
	sub.SetLinger(0)
	if err := sub.Connect(proxyAddr); err != nil {
		log.Println("[MAIN] warning: conectar SUB ao proxy falhou:", err)
	} else {
		if err := sub.SetSubscribe("servers"); err != nil {
			log.Println("[MAIN] warning: Subscribe(servers) falhou:", err)
		} else {
			log.Println("[MAIN] SUB(servers) conectado a", proxyAddr)
		}
	}

	// go routines
	go repLoop(rep)
	go publishLoop(pub)
	go sendHeartbeatLoop()
	go serversSubLoop(sub)

	// REGISTRA NO REF
	requestRank()

	list, err := requestList()
	if err != nil {
		log.Println("[REF] request list erro:", err)
	} else {
		log.Println("[REF] lista inicial:", list)
	}

	coord, err := determineCoordinator()
	if err == nil {
		currentCoordinatorMu.Lock()
		currentCoordinator = coord
		currentCoordinatorMu.Unlock()
		log.Println("[MAIN] coordenador inicial:", coord)
	} else {
		log.Println("[MAIN] determineCoordinator erro:", err)
	}

	select {}
}
