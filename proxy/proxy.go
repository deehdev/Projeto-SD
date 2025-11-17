package main

import (
    "log"

    zmq "github.com/pebbe/zmq4"
)

func main() {
    // ------------------------------
    // CONTEXTO
    // ------------------------------
    ctx, err := zmq.NewContext()
    if err != nil {
        log.Fatal("Erro criando contexto ZMQ:", err)
    }
    defer ctx.Term()

    // ------------------------------
    // SOCKET XSUB ← (servidores PUB)
    // ------------------------------
    xsub, err := ctx.NewSocket(zmq.XSUB)
    if err != nil {
        log.Fatal("Erro criando XSUB:", err)
    }
    defer xsub.Close()

    if err := xsub.Bind("tcp://*:5557"); err != nil {
        log.Fatal("Erro ao bind XSUB (porta 5557):", err)
    }

    // ------------------------------
    // SOCKET XPUB → (clients SUB)
    // ------------------------------
    xpub, err := ctx.NewSocket(zmq.XPUB)
    if err != nil {
        log.Fatal("Erro criando XPUB:", err)
    }
    defer xpub.Close()

    // Habilita logs de assinatura (corrigido para SetXpubVerbose)
    xpub.SetXpubVerbose(1)

    if err := xpub.Bind("tcp://*:5558"); err != nil {
        log.Fatal("Erro ao bind XPUB (porta 5558):", err)
    }

    log.Println("🚀 Proxy XPUB/XSUB iniciado: 5557 <-> 5558")

    // ------------------------------
    // SUBSCRIBE GLOBAL (wildcard)
    // ------------------------------
    _, err = xsub.SendBytes([]byte{1}, 0)  // corrigido para capturar os dois retornos
    if err != nil {
        log.Println("⚠️  Não foi possível enviar subscrição global:", err)
    } else {
        log.Println("📌 Subscrição global aplicada (receber todos os tópicos)")
    }

    // ------------------------------
    // PROXY STEERABLE
    // ------------------------------
    for {
        err = zmq.ProxySteerable(xsub, xpub, nil, nil)
        if err != nil {
            log.Println("⚠️ ZMQ Proxy retornou erro, reiniciando:", err)
        }
    }
}
