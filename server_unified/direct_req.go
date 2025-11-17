package main

import (
    "encoding/json"
    "time"

    zmq "github.com/pebbe/zmq4"
)

func directReqZMQJSON(addr string, env Envelope, timeout time.Duration) (Envelope, error) {

    out, _ := json.Marshal(env)

    sock, _ := zmq.NewSocket(zmq.REQ)
    sock.Connect(addr)
    sock.Send(string(out), 0)

    sock.SetRcvtimeo(timeout)

    raw, err := sock.Recv(0)
    if err != nil {
        return Envelope{}, err
    }

    var rep Envelope
    json.Unmarshal([]byte(raw), &rep)
    return rep, nil
}

func directReqZMQ(addr string, env Envelope, timeout time.Duration) (Envelope, error) {

    raw := msgpackMarshal(env)

    sock, _ := zmq.NewSocket(zmq.REQ)
    sock.Connect(addr)
    sock.SendBytes(raw, 0)

    sock.SetRcvtimeo(timeout)

    msg, err := sock.RecvBytes(0)
    if err != nil {
        return Envelope{}, err
    }

    rep := msgpackUnmarshal(msg)
    return rep, nil
}
