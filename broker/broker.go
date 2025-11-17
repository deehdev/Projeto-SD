package main

import (
    "log"
    "github.com/pebbe/zmq4"
)

func main() {
    ctx, err := zmq4.NewContext()
    if err != nil {
        log.Fatal(err)
    }
    defer ctx.Term()

    frontend, err := ctx.NewSocket(zmq4.XSUB)
    if err != nil {
        log.Fatal(err)
    }
    defer frontend.Close()

    backend, err := ctx.NewSocket(zmq4.XPUB)
    if err != nil {
        log.Fatal(err)
    }
    defer backend.Close()

    // Bind nos ports
    err = frontend.Bind("tcp://*:5557")
    if err != nil {
        log.Fatal(err)
    }

    err = backend.Bind("tcp://*:5558")
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Broker iniciado: XPUB ↔ XSUB  (5557 <-> 5558)")

    // Loop de proxy
    err = zmq4.Proxy(frontend, backend, nil)
    if err != nil {
        log.Fatal(err)
    }
}