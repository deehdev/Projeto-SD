package main

import (
    "log"
)

func publishReplicationEvent(le LogEntry) {
    env := Envelope{
        Service:   "replicate",
        Timestamp: le.Timestamp,
        Clock:     le.Clock,
        Data: map[string]interface{}{
            "id":   le.ID,
            "type": le.Type,
            "data": le.Data,
        },
    }

    raw := msgpackMarshal(env)

    pubSocketMu.Lock()
    if pubSocket != nil {
        pubSocket.SendMessage("replicate", raw)
    }
    pubSocketMu.Unlock()
}

func applyReplication(env Envelope) {
    id := env.Data["id"].(string)

    logsMu.Lock()
    for _, l := range logs {
        if l.ID == id {
            logsMu.Unlock()
            return
        }
    }

    newL := LogEntry{
        ID:        id,
        Type:      env.Data["type"].(string),
        Data:      env.Data["data"].(map[string]interface{}),
        Timestamp: env.Timestamp,
        Clock:     env.Clock,
    }

    logs = append(logs, newL)
    logsMu.Unlock()

    persistLogs()

    log.Printf("[REPLICAÇÃO] Aplicado log %s", id)
}
