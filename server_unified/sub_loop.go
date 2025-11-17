package main

import (
    "log"
    zmq "github.com/pebbe/zmq4"
)

func subLoop(sub *zmq.Socket) {
    for {
        parts, err := sub.RecvMessageBytes(0)
        if err != nil {
            log.Println("[SUB LOOP][ERRO] recv:", err)
            continue
        }

        if len(parts) < 2 {
            continue
        }

        topic := string(parts[0])
        env, err := decodeEnvelope(parts[1])
        if err != nil {
            continue
        }

        updateClock(env.Clock)

        switch topic {
        case "replicate":
            applyReplication(env)
        case "servers":
            applyCoordinatorUpdate(env)
        }
    }
}