package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	zmq "github.com/pebbe/zmq4"
)

type Message struct {
	Service string                 `json:"service"`
	Data    map[string]interface{} `json:"data"`
}

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// --------------------------------------
//         VARIÁVEIS GLOBAIS
// --------------------------------------

var (
	usersMu    sync.Mutex
	channelsMu sync.Mutex

	users    []string
	channels []string

	dataDir = "data"
)

// --------------------------------------
//            HELPERS
// --------------------------------------

func ensureDataFolder() {
	os.MkdirAll(dataDir, 0o755)
}

func usersPath() string    { return filepath.Join(dataDir, "users.json") }
func channelsPath() string { return filepath.Join(dataDir, "channels.json") }

// valida nomes (usuário e canal)
var validName = regexp.MustCompile(`^[a-z0-9_\-]{3,32}$`)

func normalizeName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	return s
}

func isValidName(s string) bool {
	return validName.MatchString(s)
}

// --------------------------------------
//            USERS
// --------------------------------------

func loadUsers() {
	ensureDataFolder()

	path := usersPath()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			users = []string{"alice", "bob"}
			saveUsers()
			return
		}
		log.Printf("Warning loading users: %v", err)
		return
	}

	json.Unmarshal(b, &users)
}

func saveUsers() {
	usersMu.Lock()
	defer usersMu.Unlock()

	b, _ := json.MarshalIndent(users, "", "  ")
	os.WriteFile(usersPath(), b, 0o644)
}

func handleLogin(data map[string]interface{}) map[string]interface{} {
	raw, _ := data["user"].(string)
	user := normalizeName(raw)

	if !isValidName(user) {
		return map[string]interface{}{
			"service": "login",
			"data": map[string]interface{}{
				"status":    "erro",
				"descricao": "nome de usuário inválido",
				"timestamp": nowISO(),
			},
		}
	}

	usersMu.Lock()
	for _, u := range users {
		if u == user {
			usersMu.Unlock()
			return map[string]interface{}{
				"service": "login",
				"data": map[string]interface{}{
					"status":    "erro",
					"descricao": "usuário já existe",
					"timestamp": nowISO(),
				},
			}
		}
	}
	users = append(users, user)
	usersMu.Unlock()

	saveUsers()
	log.Printf("Novo usuário criado: %s", user)

	return map[string]interface{}{
		"service": "login",
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

// --------------------------------------
//            CHANNELS
// --------------------------------------

func loadChannels() {
	ensureDataFolder()

	path := channelsPath()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			channels = []string{"general"}
			saveChannels()
			return
		}
		log.Printf("Warning loading channels: %v", err)
		return
	}

	json.Unmarshal(b, &channels)
}

func saveChannels() {
	channelsMu.Lock()
	defer channelsMu.Unlock()

	b, _ := json.MarshalIndent(channels, "", "  ")
	os.WriteFile(channelsPath(), b, 0o644)
}

func handleChannel(data map[string]interface{}) map[string]interface{} {
	raw, _ := data["channel"].(string)
	ch := normalizeName(raw)

	if !isValidName(ch) {
		return map[string]interface{}{
			"service": "channel",
			"data": map[string]interface{}{
				"status":    "erro",
				"descricao": "nome de canal inválido",
				"timestamp": nowISO(),
			},
		}
	}

	channelsMu.Lock()
	for _, c := range channels {
		if c == ch {
			channelsMu.Unlock()
			return map[string]interface{}{
				"service": "channel",
				"data": map[string]interface{}{
					"status":    "erro",
					"descricao": "canal já existe",
					"timestamp": nowISO(),
				},
			}
		}
	}

	channels = append(channels, ch)
	channelsMu.Unlock()

	saveChannels()
	log.Printf("Novo canal criado: %s", ch)

	return map[string]interface{}{
		"service": "channel",
		"data": map[string]interface{}{
			"status":    "sucesso",
			"timestamp": nowISO(),
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

// --------------------------------------
//                MAIN
// --------------------------------------

func main() {
	loadUsers()
	loadChannels()

	repAddr := os.Getenv("ZMQ_REP_ADDR")
	if repAddr == "" {
		repAddr = "tcp://*:5555"
	}

	context, _ := zmq.NewContext()
	defer context.Term()

	rep, err := context.NewSocket(zmq.REP)
	if err != nil {
		log.Fatalf("Erro criando REP socket: %v", err)
	}
	defer rep.Close()

	if err := rep.Bind(repAddr); err != nil {
		log.Fatalf("Erro no bind: %v", err)
	}

	log.Printf("Server iniciado em %s", repAddr)

	for {
		msgBytes, err := rep.RecvBytes(0)
		if err != nil {
			log.Printf("Erro recv: %v", err)
			continue
		}

		var req Message
		json.Unmarshal(msgBytes, &req)

		var resp map[string]interface{}
		switch req.Service {
		case "login":
			resp = handleLogin(req.Data)
		case "users":
			resp = handleUsers(req.Data)
		case "channel":
			resp = handleChannel(req.Data)
		case "channels":
			resp = handleChannels(req.Data)
		default:
			resp = map[string]interface{}{
				"service": req.Service,
				"data": map[string]interface{}{
					"status":    "erro",
					"descricao": "serviço desconhecido",
					"timestamp": nowISO(),
				},
			}
		}

		out, _ := json.Marshal(resp)
		rep.SendBytes(out, 0)
	}
}