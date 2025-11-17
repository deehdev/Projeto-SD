package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pebbe/zmq4"
	"github.com/vmihailenco/msgpack/v5"
)

// --------------------
// CONFIG
// --------------------
const dataDir = "/app/data" // Use this if running in container; change to "./data" for local runs
const logsFile = dataDir + "/logs.json"
const usersFile = dataDir + "/users.json"
const channelsFile = dataDir + "/channels.json"
const subsFile = dataDir + "/subscriptions.json"

var (
	// State
	users         []string
	channels      []string
	subscriptions = map[string][]string{}
	logs          []map[string]interface{}
	lastSeen      = map[string]time.Time{}

	// Mutexes
	usersMu    sync.Mutex
	channelsMu sync.Mutex
	subMu      sync.Mutex
	logsMu     sync.Mutex
	lastSeenMu sync.Mutex

	// timezone
	loc *time.Location
)

// --------------------
// UTILITÁRIOS
// --------------------
func init() {
	var err error
	// prefer explicit TZ
	loc, err = time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		log.Println("Não foi possível carregar timezone America/Sao_Paulo, usando UTC:", err)
		loc = time.UTC
	}
}

func nowISO() string {
	return time.Now().In(loc).Format(time.RFC3339)
}

func normalize(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

func isValid(s string) bool {
	if s == "" {
		return false
	}
	// evita espaços e caracteres problemáticos
	if strings.ContainsAny(s, " /\\@#$%^&*()[]{}!?,;:\"'") {
		return false
	}
	return true
}

// atomic write helper: create dir, write temp file then rename
func saveJSON(path string, data interface{}) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		// best effort
	}
	f.Close()
	// atomic rename
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return nil
}

func loadJSON(path string, dest interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		// Not fatal; file may not exist
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	if err := dec.Decode(dest); err != nil {
		return err
	}
	return nil
}

// --------------------
// LOG
// --------------------
func addLog(entry map[string]interface{}) {
	logsMu.Lock()
	defer logsMu.Unlock()
	entry["timestamp"] = nowISO()
	logs = append(logs, entry)
	if err := saveJSON(logsFile, logs); err != nil {
		log.Println("Erro salvando logs:", err)
	}
}

// --------------------
// HELPERS (paths)
func usersPath() string         { return usersFile }
func channelsPath() string      { return channelsFile }
func subscriptionsPath() string { return subsFile }

// --------------------
// CHECK USER ONLINE
// --------------------
func isLogged(user string) bool {
	user = normalize(user)
	usersMu.Lock()
	defer usersMu.Unlock()
	for _, u := range users {
		if u == user {
			return true
		}
	}
	return false
}

// --------------------
// HANDLERS
// --------------------
func handleLogin(data map[string]interface{}) map[string]interface{} {
	raw := fmt.Sprintf("%v", data["user"])
	user := normalize(raw)

	if !isValid(user) {
		return errorResp("login", "usuário inválido")
	}

	usersMu.Lock()
	// check exists
	for _, u := range users {
		if u == user {
			usersMu.Unlock()
			return errorResp("login", "usuário já existe")
		}
	}
	// add
	users = append(users, user)
	usersMu.Unlock()

	// ensure subscription map entry
	subMu.Lock()
	if _, ok := subscriptions[user]; !ok {
		subscriptions[user] = []string{}
	}
	subMu.Unlock()

	lastSeenMu.Lock()
	lastSeen[user] = time.Now()
	lastSeenMu.Unlock()

	// persist
	if err := saveJSON(usersPath(), users); err != nil {
		log.Println("Erro salvando users:", err)
	}
	if err := saveJSON(subscriptionsPath(), subscriptions); err != nil {
		log.Println("Erro salvando subscriptions:", err)
	}

	addLog(map[string]interface{}{
		"type":  "system",
		"user":  user,
		"event": "login",
	})

	log.Println("Usuário logado:", user)

	return successResp("login")
}

func handleHeartbeat(data map[string]interface{}) map[string]interface{} {
	raw := fmt.Sprintf("%v", data["user"])
	user := normalize(raw)

	if !isLogged(user) {
		return errorRespWithMessage("heartbeat", "usuário não está logado")
	}

	lastSeenMu.Lock()
	lastSeen[user] = time.Now()
	lastSeenMu.Unlock()

	return map[string]interface{}{
		"service": "heartbeat",
		"data": map[string]interface{}{
			"status":    "sucesso",
			"timestamp": nowISO(),
		},
	}
}

func handleUsers(data map[string]interface{}) map[string]interface{} {
	usersMu.Lock()
	list := append([]string{}, users...)
	usersMu.Unlock()

	return map[string]interface{}{
		"service": "users",
		"data": map[string]interface{}{
			"timestamp": nowISO(),
			"users":     list,
		},
	}
}

func handleChannels(data map[string]interface{}) map[string]interface{} {
	channelsMu.Lock()
	list := append([]string{}, channels...)
	channelsMu.Unlock()

	return map[string]interface{}{
		"service": "channels",
		"data": map[string]interface{}{
			"timestamp": nowISO(),
			"channels":  list,
		},
	}
}

func handleCreateChannel(data map[string]interface{}) map[string]interface{} {
	raw := fmt.Sprintf("%v", data["channel"])
	ch := normalize(raw)

	if !isValid(ch) {
		return errorResp("channel", "nome inválido")
	}

	channelsMu.Lock()
	for _, c := range channels {
		if c == ch {
			channelsMu.Unlock()
			return errorResp("channel", "canal já existe")
		}
	}
	channels = append(channels, ch)
	channelsMu.Unlock()

	if err := saveJSON(channelsPath(), channels); err != nil {
		log.Println("Erro salvando canais:", err)
	}

	addLog(map[string]interface{}{
		"type":    "system",
		"event":   "create_channel",
		"channel": ch,
	})

	return map[string]interface{}{
		"service": "channel",
		"data": map[string]interface{}{
			"status":    "sucesso",
			"channel":   ch,
			"timestamp": nowISO(),
		},
	}
}

func handleSubscribe(data map[string]interface{}) map[string]interface{} {
	user := normalize(fmt.Sprintf("%v", data["user"]))
	topic := normalize(fmt.Sprintf("%v", data["channel"]))

	if !isLogged(user) {
		return errorResp("subscribe", "usuário não existe")
	}

	channelsMu.Lock()
	found := false
	for _, c := range channels {
		if c == topic {
			found = true
			break
		}
	}
	channelsMu.Unlock()
	if !found {
		return errorResp("subscribe", "canal não existe")
	}

	subMu.Lock()
	for _, s := range subscriptions[user] {
		if s == topic {
			subMu.Unlock()
			return errorResp("subscribe", "já inscrito")
		}
	}
	subscriptions[user] = append(subscriptions[user], topic)
	subMu.Unlock()

	if err := saveJSON(subscriptionsPath(), subscriptions); err != nil {
		log.Println("Erro salvando subscriptions:", err)
	}

	addLog(map[string]interface{}{
		"type":    "system",
		"user":    user,
		"event":   "subscribe",
		"channel": topic,
	})

	return successResp("subscribe")
}

func handleUnsubscribe(data map[string]interface{}) map[string]interface{} {
	user := normalize(fmt.Sprintf("%v", data["user"]))
	topic := normalize(fmt.Sprintf("%v", data["channel"]))

	subMu.Lock()
	subs := subscriptions[user]
	newSubs := []string{}
	found := false
	for _, s := range subs {
		if s == topic {
			found = true
			continue
		}
		newSubs = append(newSubs, s)
	}
	if !found {
		subMu.Unlock()
		return errorResp("unsubscribe", "não inscrito neste canal")
	}
	subscriptions[user] = newSubs
	subMu.Unlock()

	if err := saveJSON(subscriptionsPath(), subscriptions); err != nil {
		log.Println("Erro salvando subscriptions:", err)
	}

	addLog(map[string]interface{}{
		"type":    "system",
		"user":    user,
		"event":   "unsubscribe",
		"channel": topic,
	})

	return successResp("unsubscribe")
}

func handlePublish(data map[string]interface{}, pub *zmq4.Socket) map[string]interface{} {
	ch := normalize(fmt.Sprintf("%v", data["channel"]))
	msg := fmt.Sprintf("%v", data["message"])
	user := normalize(fmt.Sprintf("%v", data["user"]))

	channelsMu.Lock()
	ok := false
	for _, c := range channels {
		if c == ch {
			ok = true
			break
		}
	}
	channelsMu.Unlock()
	if !ok {
		return errorResp("publish", "canal inexistente")
	}

	subMu.Lock()
	subs := append([]string{}, subscriptions[user]...)
	subMu.Unlock()

	allowed := false
	for _, s := range subs {
		if s == ch {
			allowed = true
			break
		}
	}
	if !allowed {
		return errorResp("publish", "você não está inscrito neste canal")
	}

	// formato de publicação: [timestamp][channel][user] message
	payload := fmt.Sprintf("[%s][%s][%s] %s", nowISO(), ch, user, msg)
	// topic = ch, message = payload
	if _, err := pub.SendMessage(ch, payload); err != nil {
		log.Println("Erro enviando publish:", err)
	}

	addLog(map[string]interface{}{
		"type":    "public",
		"user":    user,
		"channel": ch,
		"message": msg,
	})

	return successResp("publish")
}

func handleMessage(data map[string]interface{}, pub *zmq4.Socket) map[string]interface{} {
	dst := normalize(fmt.Sprintf("%v", data["dst"]))
	src := normalize(fmt.Sprintf("%v", data["src"]))
	msg := fmt.Sprintf("%v", data["message"])

	// check if destination exists
	usersMu.Lock()
	exists := false
	for _, u := range users {
		if u == dst {
			exists = true
			break
		}
	}
	usersMu.Unlock()
	if !exists {
		return errorResp("message", "usuário destino não existe")
	}

	// formato de PM: [timestamp][PM][user] message
	payload := fmt.Sprintf("[%s][PM][%s] %s", nowISO(), src, msg)
	if _, err := pub.SendMessage(dst, payload); err != nil {
		log.Println("Erro enviando PM:", err)
	}

	addLog(map[string]interface{}{
		"type":        "private",
		"user":        src,
		"destination": dst,
		"message":     msg,
		"direction":   "sent",
	})

	return successResp("message")
}

// --------------------
// RESPONSES HELPERS
// --------------------
func errorResp(service, message string) map[string]interface{} {
	return map[string]interface{}{
		"service": service,
		"data": map[string]interface{}{
			"status":    "erro",
			"message":   message,
			"timestamp": nowISO(),
		},
	}
}

func errorRespWithMessage(service, message string) map[string]interface{} {
	return errorResp(service, message)
}

func successResp(service string) map[string]interface{} {
	return map[string]interface{}{
		"service": service,
		"data": map[string]interface{}{
			"status":    "sucesso",
			"timestamp": nowISO(),
		},
	}
}

// ------------------------------------
// MEUS CANAIS
// ------------------------------------
func handleMySubs(data map[string]interface{}) map[string]interface{} {
	user := normalize(fmt.Sprintf("%v", data["user"]))

	// Se o usuário não existir, retorna erro claro
	if !isLogged(user) {
		return errorResp("mysubs", "usuário não está logado")
	}

	subMu.Lock()
	subs, exists := subscriptions[user]
	subMu.Unlock()

	// Se não existir no mapa (primeiro login), retorna lista vazia
	if !exists {
		subs = []string{}
	}

	return map[string]interface{}{
		"service": "mysubs",
		"data": map[string]interface{}{
			"status":    "sucesso",
			"timestamp": nowISO(),
			"subs":      subs,
		},
	}
}

// ------------------------------------
// CANAIS AUTORIZADOS PARA PUBLICAÇÃO
// ------------------------------------
func handlePublishable(data map[string]interface{}) map[string]interface{} {
	user := normalize(fmt.Sprintf("%v", data["user"]))

	// Verifica se o usuário fez login / existe no sistema
	if !isLogged(user) {
		return errorResp("publishable", "usuário não está logado")
	}

	subMu.Lock()
	subs, exists := subscriptions[user]
	subMu.Unlock()

	if !exists {
		subs = []string{}
	}

	return map[string]interface{}{
		"service": "publishable",
		"data": map[string]interface{}{
			"status":      "sucesso",
			"timestamp":   nowISO(),
			"publishable": subs,
		},
	}
}

// --------------------
// BACKGROUND: REMOVE INACTIVE USERS
// --------------------
func startCleaner(repSaveInterval time.Duration, timeout time.Duration) {
	go func() {
		for {
			time.Sleep(repSaveInterval)
			now := time.Now()

			lastSeenMu.Lock()
			usersMu.Lock()
			subMu.Lock()

			newUsers := []string{}
			removed := []string{}

			for _, u := range users {
				t, ok := lastSeen[u]
				if !ok || now.Sub(t) > timeout {
					// remove user
					removed = append(removed, u)
					delete(subscriptions, u)
					delete(lastSeen, u)
					addLog(map[string]interface{}{
						"type":  "system",
						"event": "auto_logout",
						"user":  u,
					})
				} else {
					newUsers = append(newUsers, u)
				}
			}

			users = newUsers

			// persist state if changes
			if len(removed) > 0 {
				if err := saveJSON(usersPath(), users); err != nil {
					log.Println("Erro salvando users (cleaner):", err)
				}
				if err := saveJSON(subscriptionsPath(), subscriptions); err != nil {
					log.Println("Erro salvando subscriptions (cleaner):", err)
				}
				log.Println("Usuários removidos por timeout:", removed)
			}

			subMu.Unlock()
			usersMu.Unlock()
			lastSeenMu.Unlock()
		}
	}()
}

// --------------------
// MAIN
// --------------------
func main() {
	// Ensure data dir exists
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatal("Não foi possível criar data dir:", err)
	}

	// Try loading persisted state (ignore errors if files don't exist)
	_ = loadJSON(usersPath(), &users)
	_ = loadJSON(channelsPath(), &channels)
	_ = loadJSON(subscriptionsPath(), &subscriptions)
	_ = loadJSON(logsFile, &logs)

	// ensure non-nil
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
		logs = []map[string]interface{}{}
	}

	// Initialize lastSeen for known users (set to now)
	lastSeenMu.Lock()
	for _, u := range users {
		lastSeen[u] = time.Now()
	}
	lastSeenMu.Unlock()

	// Start cleaner: every 15s, remove users inactive > 60s (configurable)
	startCleaner(15*time.Second, 60*time.Second)

	// ZMQ setup
	ctx, err := zmq4.NewContext()
	if err != nil {
		log.Fatal("Erro criando contexto ZMQ:", err)
	}
	defer ctx.Term()

	rep, err := ctx.NewSocket(zmq4.REP)
	if err != nil {
		log.Fatal("Erro criando socket REP:", err)
	}
	defer rep.Close()

	pub, err := ctx.NewSocket(zmq4.PUB)
	if err != nil {
		log.Fatal("Erro criando socket PUB:", err)
	}
	defer pub.Close()

	// Bind/Connect: the server listens REP on 5555, and PUB connects to proxy:5557 (as in your setup)
	if err := rep.Bind("tcp://*:5555"); err != nil {
		log.Fatal("Erro bind REP:", err)
	}
	if err := pub.Connect("tcp://proxy:5557"); err != nil {
		log.Println("Aviso: não conseguiu conectar PUB para proxy:5557 (talvez o proxy ainda não esteja pronto):", err)
	}

	log.Println("Servidor iniciado (MessagePack REQ/REP)!")

	// Main loop (MessagePack REQ/REP)
	for {
		raw, err := rep.RecvBytes(0) // recebe bytes binários (MessagePack)
		if err != nil {
			log.Println("Erro recebendo requisição:", err)
			continue
		}

		var req map[string]interface{}
		if err := msgpack.Unmarshal(raw, &req); err != nil {
			log.Println("MessagePack inválido recebido:", err)
			resp := errorResp("error", "MessagePack inválido")
			outBytes, _ := msgpack.Marshal(resp)
			if _, err := rep.SendBytes(outBytes, 0); err != nil {
				log.Println("Erro enviando resposta REP (invalid msgpack):", err)
			}
			continue
		}

		service := fmt.Sprintf("%v", req["service"])
		var data map[string]interface{}
		if req["data"] != nil {
			if d, ok := req["data"].(map[string]interface{}); ok {
				data = d
			} else {
				data = map[string]interface{}{}
			}
		} else {
			data = map[string]interface{}{}
		}

		var resp map[string]interface{}

		switch service {
		case "login":
			resp = handleLogin(data)
		case "heartbeat":
			resp = handleHeartbeat(data)
		case "users":
			resp = handleUsers(data)
		case "channels":
			resp = handleChannels(data)
		case "channel":
			resp = handleCreateChannel(data)
		case "subscribe":
			resp = handleSubscribe(data)
		case "unsubscribe":
			resp = handleUnsubscribe(data)
		case "mysubs":
			resp = handleMySubs(data)
		case "publishable":
			resp = handlePublishable(data)
		case "publish":
			resp = handlePublish(data, pub)
		case "message":
			resp = handleMessage(data, pub)
		default:
			resp = errorResp("error", "serviço desconhecido")
		}

		outBytes, _ := msgpack.Marshal(resp)
		if _, err := rep.SendBytes(outBytes, 0); err != nil {
			log.Println("Erro enviando resposta REP:", err)
		}
	}
}
