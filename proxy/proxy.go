package main

import (
    "log"
    zmq "github.com/pebbe/zmq4"
)

func main() {
    xsub, _ := zmq.NewSocket(zmq.XSUB)
    xpub, _ := zmq.NewSocket(zmq.XPUB)

    xsub.Bind("tcp://*:5557")
    xpub.Bind("tcp://*:5558")

    log.Println("[PROXY] XPUB/XSUB ativo nas portas 5557 (XSUB) e 5558 (XPUB)")
    zmq.Proxy(xsub, xpub, nil)
}
