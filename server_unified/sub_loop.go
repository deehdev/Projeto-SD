package main

import (
    "log"
    zmq "github.com/pebbe/zmq4"
)

// Loop SUB do servidor — recebe replicação e mensagens de coordenação
func subLoop(sub *zmq.Socket) {
    for {
        parts, err := sub.RecvMessageBytes(0)
        if err != nil {
            log.Println("[SUB LOOP][ERRO] recv:", err)
            continue
        }

        if len(parts) < 2 {
            log.Println("[SUB LOOP][WARN] Mensagem inválida recebida")
            continue
        }

        topic := string(parts[0])
        payload := parts[1]

        env, err := decodeEnvelope(payload)
        if err != nil {
            log.Println("[SUB LOOP][ERRO] decode:", err)
            continue
        }

        updateClock(env.Clock)

        switch topic {

        case "replicate":
            applyReplication(env)

        case "servers":
            applyCoordinatorUpdate(env)

        default:
            log.Println("[SUB LOOP][WARN] Tópico desconhecido:", topic)
        }
    }
}
