package main

import (
    "sync"

    zmq "github.com/pebbe/zmq4"
)

var (
    // Users
    users   []string
    usersMu sync.Mutex

    // Channels
    channels   []string
    channelsMu sync.Mutex

    // Subscriptions
    subscriptions   map[string][]string
    subMu           sync.Mutex

    // Logs
    logs   []LogEntry
    logsMu sync.Mutex

    // Logical clock
    logicalClock   int
    logicalClockMu sync.Mutex

    // Coordenador
    currentCoordinator   string
    currentCoordinatorMu sync.Mutex

    // ENV
    serverName   string
    repPort      int
    refAddr      string
    proxyPubAddr string

    // PUB socket global (para replicação)
    pubSocket    *zmq.Socket
    pubSocketMu  sync.Mutex
)
