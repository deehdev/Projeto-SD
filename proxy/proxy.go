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

    // Recebe mensagens dos servidores (PUB)
    xsub, err := ctx.NewSocket(zmq.XSUB)
    if err != nil {
        log.Fatal("Erro criando XSUB:", err)
    }
    defer xsub.Close()

    // Envia para bots/clients (SUB)
    xpub, err := ctx.NewSocket(zmq.XPUB)
    if err != nil {
        log.Fatal("Erro criando XPUB:", err)
    }
    defer xpub.Close()

    // Bind nas portas de entrada e saída
    if err := xsub.Bind("tcp://*:5557"); err != nil {
        log.Fatal("Erro ao bind XSUB:", err)
    }

    if err := xpub.Bind("tcp://*:5558"); err != nil {
        log.Fatal("Erro ao bind XPUB:", err)
    }

    log.Println("Proxy XPUB/XSUB iniciado na rota 5557 <-> 5558")

    // Enviar subscrição inicial (necessário para iniciar fluxo)
    // '\x01' = SUBSCRIBE ALL
    _, err = xsub.SendMessage([]byte{1})
    if err != nil {
        log.Println("Aviso: não foi possível enviar subscrição inicial:", err)
    }

    // LOOP DO PROXY ZMQ – transfere os bytes puros
    for {
        err = zmq.Proxy(xsub, xpub, nil)
        if err != nil {
            log.Println("ZMQ Proxy retornou erro, reiniciando:", err)
        }
    }
}
