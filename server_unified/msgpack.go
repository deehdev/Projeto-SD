package main

import (
    "log"

    "github.com/vmihailenco/msgpack/v5"
)

func msgpackMarshal(v interface{}) []byte {
    raw, err := msgpack.Marshal(v)
    if err != nil {
        log.Printf("[MSGPACK][ERRO] %v", err)
        return []byte{}
    }
    return raw
}

func msgpackUnmarshal(data []byte) Envelope {
    var env Envelope
    err := msgpack.Unmarshal(data, &env)
    if err != nil {
        log.Printf("[MSGPACK][ERRO] %v", err)
        return Envelope{}
    }
    return env
}
