package main

import (
	"log"
	"os"
	"time"

	zmq "github.com/pebbe/zmq4"
)

func main() {

	xsubAddr := os.Getenv("XSUB_ADDR")
	if xsubAddr == "" {
		xsubAddr = "tcp://*:5557"
	}

	xpubAddr := os.Getenv("XPUB_ADDR")
	if xpubAddr == "" {
		xpubAddr = "tcp://*:5558"
	}

	log.Println("───────────────────────────────────────────────")
	log.Println("     🚀 INICIANDO PROXY ZMQ XSUB ⇄ XPUB")
	log.Println("───────────────────────────────────────────────")
	log.Printf("XSUB: servidores publicam →   %s", xsubAddr)
	log.Printf("XPUB: clientes se inscrevem → %s", xpubAddr)
	log.Println("───────────────────────────────────────────────")

	ctx, err := zmq.NewContext()
	if err != nil {
		log.Fatal("Erro criando contexto:", err)
	}
	defer ctx.Term()

	// ---------------------------
	// XSUB → onde os servidores publicam
	// ---------------------------
	xsub, err := ctx.NewSocket(zmq.XSUB)
	if err != nil {
		log.Fatal("Erro criando socket XSUB:", err)
	}
	defer xsub.Close()

	if err := xsub.Bind(xsubAddr); err != nil {
		log.Fatal("Erro bind XSUB:", err)
	}

	// ---------------------------
	// XPUB → onde os clientes fazem SUBSCRIBE
	// ---------------------------
	xpub, err := ctx.NewSocket(zmq.XPUB)
	if err != nil {
		log.Fatal("Erro criando socket XPUB:", err)
	}
	defer xpub.Close()

	// 🔥 ESSENCIAL: libera eventos de inscrição
	xpub.SetXpubVerbose(1)

	if err := xpub.Bind(xpubAddr); err != nil {
		log.Fatal("Erro bind XPUB:", err)
	}

	log.Println("✅ Proxy XSUB/XPUB pronto")

	// ---------------------------------------------------------
	// THREAD PARA MONITORAR INSCRIÇÕES RECEBIDAS PELO XPUB
	// ---------------------------------------------------------
	go func() {
		for {
			msg, err := xpub.RecvMessageBytes(0)
			if err != nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			if len(msg) == 0 {
				continue
			}

			event := msg[0][0]
			topic := ""
			if len(msg[0]) > 1 {
				topic = string(msg[0][1:])
			}

			if event == 1 {
				log.Printf("🔔 XPUB → cliente SE INSCREVE em tópico [%s]", topic)
			} else if event == 0 {
				log.Printf("🔕 XPUB → cliente CANCELA tópico [%s]", topic)
			}
		}
	}()

	// ---------------------------------------------------------
	// LOOP DO PROXY
	// ---------------------------------------------------------
	for {
		log.Println("🔄 Proxy ativo: roteando mensagens...")
		err := zmq.Proxy(xsub, xpub, nil)

		log.Println("⚠️ Proxy terminou com erro:", err)
		time.Sleep(500 * time.Millisecond)
	}
}
