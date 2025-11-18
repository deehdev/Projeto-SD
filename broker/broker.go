package main

import (
	log "log"
	zmq "github.com/pebbe/zmq4"
)

func main() {
	// Criar contexto
	ctx, err := zmq.NewContext()
	if err != nil {
		log.Fatal("[BROKER] Erro criando contexto:", err)
	}

	// FRONTEND = ROUTER (clientes/bots)
	frontend, err := ctx.NewSocket(zmq.ROUTER)
	if err != nil {
		log.Fatal("[BROKER] Erro criando ROUTER:", err)
	}
	defer frontend.Close()

	if err := frontend.Bind("tcp://*:5555"); err != nil {
		log.Fatal("[BROKER] Erro bind ROUTER (porta 5555):", err)
	}

	// BACKEND = DEALER (servidores)
	backend, err := ctx.NewSocket(zmq.DEALER)
	if err != nil {
		log.Fatal("[BROKER] Erro criando DEALER:", err)
	}
	defer backend.Close()

	// ✅ Bind na porta 5556 para coincidir com Docker Compose
	if err := backend.Bind("tcp://*:5556"); err != nil {
		log.Fatal("[BROKER] Erro bind DEALER (porta 5556):", err)
	}

	log.Println("[BROKER] REQ-REP ativo — ROUTER 5555 <-> DEALER 5556")

	// Proxy ZMQ (round-robin)
	err = zmq.Proxy(frontend, backend, nil)
	if err != nil {
		log.Fatal("[BROKER] Proxy terminou inesperadamente:", err)
	}
}
