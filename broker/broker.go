package main

import (
	"log"
	"time"

	zmq "github.com/pebbe/zmq4"
)

func StartBroker() error {
	// Cria contexto ZeroMQ
	ctx, err := zmq.NewContext()
	if err != nil {
		return err
	}
	defer ctx.Term()

	// XSUB: recebe mensagens de publishers
	xsub, err := ctx.NewSocket(zmq.XSUB)
	if err != nil {
		return err
	}
	defer xsub.Close()
	if err := xsub.Bind("tcp://*:5557"); err != nil {
		return err
	}

	// XPUB: envia para subscribers
	xpub, err := ctx.NewSocket(zmq.XPUB)
	if err != nil {
		return err
	}
	defer xpub.Close()
	if err := xpub.Bind("tcp://*:5558"); err != nil {
		return err
	}

	log.Println("Broker XSUB/XPUB rodando em: tcp://*:5557 <-> tcp://*:5558")

	// Loop principal do proxy
	if err := zmq.Proxy(xsub, xpub, nil); err != nil {
		log.Printf("Proxy encerrado com erro: %v", err)
		return err
	}

	return nil
}

func main() {
	log.Println("Iniciando o broker de chat...")

	for {
		err := StartBroker()
		if err != nil {
			log.Printf("Erro crítico no broker ZeroMQ. Reiniciando em 5s: %v", err)
			time.Sleep(5 * time.Second)
		}
	}
}