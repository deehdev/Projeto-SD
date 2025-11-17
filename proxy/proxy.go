package main

import (
	"log"

	zmq "github.com/pebbe/zmq4"
)

func main() {
	ctx, err := zmq.NewContext()
	if err != nil {
		log.Fatal("Erro criando contexto ZMQ:", err)
	}
	defer ctx.Term()

	// Socket XSUB recebe dos servidores (PUB)
	xsub, err := ctx.NewSocket(zmq.XSUB)
	if err != nil {
		log.Fatal("Erro criando XSUB:", err)
	}
	defer xsub.Close()

	// Socket XPUB envia para os bots/clients (SUB)
	xpub, err := ctx.NewSocket(zmq.XPUB)
	if err != nil {
		log.Fatal("Erro criando XPUB:", err)
	}
	defer xpub.Close()

	// Bind nas portas padrão do seu sistema
	err = xsub.Bind("tcp://*:5557")
	if err != nil {
		log.Fatal("Erro ao bind XSUB:", err)
	}

	err = xpub.Bind("tcp://*:5558")
	if err != nil {
		log.Fatal("Erro ao bind XPUB:", err)
	}

	log.Println("Proxy XPUB/XSUB iniciado na rota 5557 <-> 5558")

	// Importante: permitir que XPUB receba mensagens de subscrição
	// e repasse isso ao XSUB automaticamente.
	// Isso garante que qualquer SUB novo receba publicações de qualquer tópico.
	//
	// '\x01' = subscribe
	// '\x00' = unsubscribe
	//
	// Sem isso, alguns sistemas ficam sem receber
	// mensagens até o primeiro SUB real ser enviado.
	_, err = xsub.SendMessage([]byte{1})
	if err != nil {
		log.Println("Aviso: não foi possível enviar subscrição inicial ao XSUB:", err)
	}

	// Proxy com tratamento de fallback
	for {
		err = zmq.Proxy(xsub, xpub, nil)
		if err != nil {
			log.Println("ZMQ Proxy retornou erro, reiniciando:", err)
		}
	}
}