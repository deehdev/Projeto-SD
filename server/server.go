// server.go
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	zmq "github.com/pebbe/zmq4"
	"github.com/vmihailenco/msgpack/v5"
)

type Envelope struct {
	Service string                 `msgpack:"service"`
	Data    map[string]interface{} `msgpack:"data"`
	Clock   int                    `msgpack:"clock"`
}

type RefReply struct {
	Service string                 `msgpack:"service"`
	Data    map[string]interface{} `msgpack:"data"`
	Clock   int                    `msgpack:"clock"`
}

// ----------------------- FIX para conversões numéricas -----------------------
func getInt(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case int8:
		return int(x)
	case int16:
		return int(x)
	case int32:
		return int(x)
	case int64:
		return int(x)
	case uint:
		return int(x)
	case uint8:
		return int(x)
	case uint16:
		return int(x)
	case uint32:
		return int(x)
	case uint64:
		return int(x)
	case float32:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

var (
	serverName   string
	serverRank   int
	serverAddr   string
	clock        int
	clockMutex   sync.Mutex
	coordMutex   sync.Mutex
	coordinator  string
	serversList  = map[string]map[string]interface{}{}
	serversMutex sync.Mutex

	pubSocket *zmq.Socket
	repClient *zmq.Socket

	repSrv *zmq.Socket
	reqMap = map[string]*zmq.Socket{}

	// persistence
	messages            = []Envelope{}
	msgCountSinceSync   = 0
	msgCountSinceSyncMu sync.Mutex
)

// ----------------------- helpers -----------------------
func logInfo(format string, a ...interface{}) { log.Printf("[INFO] "+format, a...) }
func logWarn(format string, a ...interface{}) { log.Printf("[WARN] "+format, a...) }
func logErr(format string, a ...interface{})  { log.Printf("[ERROR] "+format, a...) }

func incClock() int {
	clockMutex.Lock()
	clock++
	v := clock
	clockMutex.Unlock()
	return v
}

func updateClock(recv interface{}) {
	n := getInt(recv)
	clockMutex.Lock()
	if n > clock {
		clock = n
	}
	clock++
	clockMutex.Unlock()
}

func nowISO() string { return time.Now().UTC().Format(time.RFC3339) }

// ----------------------- persistence -----------------------
func saveJSON(path string, v interface{}) {
	b, _ := json.MarshalIndent(v, "", "  ")
	_ = ioutil.WriteFile(path, b, 0644)
}

func persistMessages() { saveJSON("data/messages.json", messages) }
func loadMessages() {
	b, err := ioutil.ReadFile("data/messages.json")
	if err == nil {
		_ = json.Unmarshal(b, &messages)
	}
}

// ----------------------- REF interaction -----------------------
func refRequest(req map[string]interface{}) (map[string]interface{}, error) {
	ctx, _ := zmq.NewContext()
	defer ctx.Term()
	sock, _ := ctx.NewSocket(zmq.REQ)
	defer sock.Close()

	refAddr := os.Getenv("REF_ADDR")
	if refAddr == "" {
		return nil, fmt.Errorf("REF_ADDR missing")
	}

	sock.Connect(refAddr)
	req["clock"] = incClock()

	out, _ := msgpack.Marshal(map[string]interface{}{
		"service": req["service"],
		"data":    req,
		"clock":   req["clock"],
	})

	if _, err := sock.SendBytes(out, 0); err != nil {
		return nil, err
	}

	replyBytes, err := sock.RecvBytes(0)
	if err != nil {
		return nil, err
	}

	var rep RefReply
	if err := msgpack.Unmarshal(replyBytes, &rep); err != nil {
		return nil, err
	}

	updateClock(rep.Clock)

	return rep.Data, nil
}

// ----------------------- server-to-server -----------------------
func ensureReqTo(addr string) (*zmq.Socket, error) {
	if s, ok := reqMap[addr]; ok {
		return s, nil
	}

	ctx, _ := zmq.NewContext()
	req, _ := ctx.NewSocket(zmq.REQ)
	req.SetLinger(0)

	if err := req.Connect(addr); err != nil {
		return nil, err
	}

	reqMap[addr] = req
	return req, nil
}

func sendSrvReq(addr string, service string, data map[string]interface{}, timeoutMs int) (map[string]interface{}, error) {
	sock, err := ensureReqTo(addr)
	if err != nil {
		return nil, err
	}

	env := map[string]interface{}{"service": service, "data": data, "clock": incClock()}
	out, _ := msgpack.Marshal(env)

	if _, err := sock.SendBytes(out, 0); err != nil {
		return nil, err
	}

	poller := zmq.NewPoller()
	poller.Add(sock, zmq.POLLIN)

	socks, _ := poller.Poll(time.Duration(timeoutMs) * time.Millisecond)
	if len(socks) == 0 {
		return nil, fmt.Errorf("timeout")
	}

	msg, _ := sock.RecvBytes(0)
	var rep map[string]interface{}
	_ = msgpack.Unmarshal(msg, &rep)

	// update clock
	if c, ok := rep["clock"]; ok {
		updateClock(c)
	}

	return rep, nil
}

// ----------------------- Election (Bully) -----------------------
func startElection() {
	logInfo("Iniciando eleição (Bully) — procurando servidores com rank maior...")

	serversMutex.Lock()
	targets := []struct {
		name string
		addr string
		rank int
	}{}

	for name, info := range serversList {
		r := getInt(info["rank"])
		addr := info["addr"].(string)
		if r > serverRank {
			targets = append(targets, struct {
				name string
				addr string
				rank int
			}{name, addr, r})
		}
	}
	serversMutex.Unlock()

	if len(targets) == 0 {
		declareCoordinator(serverName)
		return
	}

	gotOK := false
	for _, t := range targets {
		logInfo("Enviando 'election' para %s (%s) rank=%d", t.name, t.addr, t.rank)

		_, err := sendSrvReq(t.addr, "election", map[string]interface{}{"from": serverName}, 1500)
		if err == nil {
			gotOK = true
			logInfo("Recebi OK de %s — desistindo de ser coordenador", t.name)
			break
		}
	}

	if !gotOK {
		declareCoordinator(serverName)
	} else {
		logInfo("Aguardando anúncio do novo coordenador...")
	}
}

func declareCoordinator(name string) {
	coordMutex.Lock()
	coordinator = name
	coordMutex.Unlock()

	ann := map[string]interface{}{
		"service": "election",
		"data": map[string]interface{}{
			"coordinator": name,
			"timestamp":   nowISO(),
			"clock":       incClock(),
		},
	}

	out, _ := msgpack.Marshal(ann)
	if pubSocket != nil {
		pubSocket.SendMessage("servers", out)
	}

	logInfo("🟡 Novo coordenador: %s", name)
}

// ----------------------- SUB listener -----------------------
func startSubListener(sub *zmq.Socket) {
	for {
		parts, err := sub.RecvMessageBytes(0)
		if err != nil || len(parts) < 2 {
			continue
		}

		topic := string(parts[0])
		body := parts[1]

		if topic == "servers" {
			var ann map[string]interface{}
			msgpack.Unmarshal(body, &ann)

			if data, ok := ann["data"].(map[string]interface{}); ok {
				if coord, ok := data["coordinator"].(string); ok {
					coordMutex.Lock()
					coordinator = coord
					coordMutex.Unlock()
					logInfo("📢 Novo coordenador recebido: %s", coord)
				}
			}
		}
	}
}

// ----------------------- Server-side REQ handler -----------------------
func handleSrvReq(env Envelope) map[string]interface{} {
	switch env.Service {

	case "election":
		logInfo("Recebi pedido de eleição de %v", env.Data["from"])
		return map[string]interface{}{"election": "OK", "clock": incClock()}

	case "clock":
		return map[string]interface{}{"time": nowISO(), "clock": incClock()}

	default:
		return map[string]interface{}{"error": "unknown service", "clock": incClock()}
	}
}

// ----------------------- Server REP loop -----------------------
func startRepSrvLoop() {
	for {
		msg, err := repSrv.RecvBytes(0)
		if err != nil {
			continue
		}
		var env Envelope
		msgpack.Unmarshal(msg, &env)

		updateClock(env.Clock)

		resp := handleSrvReq(env)
		out, _ := msgpack.Marshal(resp)
		repSrv.SendBytes(out, 0)
	}
}

// ----------------------- MAIN -----------------------
func main() {
	rand.Seed(time.Now().UnixNano())

	serverName = os.Getenv("SERVER_NAME")
	if serverName == "" {
		serverName = "server-" + fmt.Sprintf("%04d", rand.Intn(10000))
	}

	serverAddr = os.Getenv("SERVER_ADDR")
	if serverAddr == "" {
		logErr("SERVER_ADDR ausente!")
		os.Exit(1)
	}

	os.MkdirAll("data", 0755)
	loadMessages()

	// ---------- ZMQ ----------
	ctx, _ := zmq.NewContext()

	repClient, _ = ctx.NewSocket(zmq.REP)
	repClient.Connect(os.Getenv("BROKER_DEALER_ADDR"))

	pubSocket, _ = ctx.NewSocket(zmq.PUB)
	pubSocket.Connect(os.Getenv("PROXY_PUB_ADDR"))

	sub, _ := ctx.NewSocket(zmq.SUB)
	sub.SetSubscribe("servers")
	sub.Connect(os.Getenv("PROXY_SUB_ADDR"))
	go startSubListener(sub)

	repSrv, _ = ctx.NewSocket(zmq.REP)
	repSrv.Bind(serverAddr)
	go startRepSrvLoop()

	// ---------- Register on REF ----------
	refResp, err := refRequest(map[string]interface{}{"service": "rank", "user": serverName, "addr": serverAddr})
	if err != nil {
		logErr("Erro no REF: %v", err)
		os.Exit(1)
	}

	serverRank = getInt(refResp["rank"])

	logInfo("Servidor iniciado → Nome=%s Rank=%d Addr=%s", serverName, serverRank, serverAddr)

	// ---------- Get full server list ----------
	listResp, _ := refRequest(map[string]interface{}{"service": "list"})

	if l, ok := listResp["list"].([]interface{}); ok {
		serversMutex.Lock()
		for _, it := range l {
			m := it.(map[string]interface{})
			name := m["name"].(string)

			serversList[name] = map[string]interface{}{
				"rank": getInt(m["rank"]),
				"addr": m["addr"].(string),
			}
		}
		serversMutex.Unlock()
	}

	// ---------- Choose highest rank as coordinator ----------
	{
		highestName := serverName
		highestRank := serverRank

		serversMutex.Lock()
		for name, info := range serversList {
			r := getInt(info["rank"])
			if r > highestRank {
				highestRank = r
				highestName = name
			}
		}
		coordinator = highestName
		serversMutex.Unlock()

		logInfo("Coordenador inicial definido: %s", coordinator)
	}

	// ---------- Heartbeat ----------
	go func() {
		for {
			refRequest(map[string]interface{}{"service": "heartbeat", "user": serverName, "addr": serverAddr})
			time.Sleep(5 * time.Second)
		}
	}()

	// ---------- Coordinator liveness check ----------
	go func() {
		for {
			time.Sleep(4 * time.Second)

			coordMutex.Lock()
			current := coordinator
			coordMutex.Unlock()

			if current == serverName {
				continue
			}

			serversMutex.Lock()
			info, ok := serversList[current]
			serversMutex.Unlock()

			if !ok {
				startElection()
				continue
			}

			addr := info["addr"].(string)

			_, err := sendSrvReq(addr, "ping", map[string]interface{}{"from": serverName}, 1200)
			if err != nil {
				logWarn("Coordenador %s parece inativo. Iniciando eleição...", current)
				startElection()
			}
		}
	}()

	// ---------- MAIN CLIENT LOOP ----------
	for {
		reqMsg, err := repClient.RecvMessageBytes(0)
		if err != nil {
			continue
		}

		var req Envelope
		msgpack.Unmarshal(reqMsg[0], &req)

		updateClock(req.Clock)

		resp := map[string]interface{}{
			"timestamp": nowISO(),
			"clock":     incClock(),
		}

		switch req.Service {

		case "login":
			user := ""
			if v := req.Data["user"]; v != nil {
				user = v.(string)
			}
			if user == "" {
				resp["status"] = "erro"
				resp["description"] = "usuário inválido"
			} else {
				resp["status"] = "sucesso"
			}

		case "channels":
			resp["channels"] = []string{"geral"}

		case "publish":
			ch := req.Data["channel"].(string)
			env := Envelope{
				Service: "publish",
				Data: map[string]interface{}{
					"user":      req.Data["user"],
					"channel":   ch,
					"message":   req.Data["message"],
					"timestamp": resp["timestamp"],
				},
				Clock: incClock(),
			}

			packed, _ := msgpack.Marshal(env)
			pubSocket.SendMessage(ch, packed)

			messages = append(messages, env)
			persistMessages()

			resp["status"] = "OK"

		case "message":
			dst := req.Data["dst"].(string)

			env := Envelope{
				Service: "message",
				Data: map[string]interface{}{
					"src":       req.Data["src"],
					"dst":       dst,
					"message":   req.Data["message"],
					"timestamp": resp["timestamp"],
				},
				Clock: incClock(),
			}

			packed, _ := msgpack.Marshal(env)
			pubSocket.SendMessage(dst, packed)

			messages = append(messages, env)
			persistMessages()

			resp["status"] = "OK"

		default:
			resp["status"] = "erro"
			resp["message"] = "serviço desconhecido"
		}

		out, _ := msgpack.Marshal(map[string]interface{}{"service": req.Service, "data": resp, "clock": incClock()})
		repClient.SendBytes(out, 0)
	}
}
