package main

import (
    "encoding/json"
    "log"

    zmq "github.com/pebbe/zmq4"
)

func main() {
    // Recebe requisições dos clientes
    rep, _ := zmq.NewSocket(zmq.REP)
    rep.Bind("tcp://*:5555")

    log.Println("[BROKER] REQ/REP ativo na porta 5555")

    for {
        msgBytes, _ := rep.RecvBytes(0)

        // Apenas encaminha ao servidor principal
        var env map[string]interface{}
        json.Unmarshal(msgBytes, &env)

        log.Printf("[BROKER] REQ recebido: %v\n", env)

        // Apenas ecoa como OK (somente parte 2)
        reply := map[string]interface{}{
            "service": env["service"],
            "data": map[string]interface{}{
                "status": "OK",
            },
        }

        respBytes, _ := json.Marshal(reply)
        rep.SendBytes(respBytes, 0)
    }
}
