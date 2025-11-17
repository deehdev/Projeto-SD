package main

import (
    "github.com/google/uuid"
)

// ---------------------------------------------
// CREATE CHANNEL
// ---------------------------------------------
func handleCreateChannel(env Envelope) Envelope {
    raw := ""
    if v, ok := env.Data["channel"].(string); ok {
        raw = v
    }
    ch := normalize(raw)

    resp := Envelope{
        Service:   "channel",
        Data:      map[string]interface{}{},
        Timestamp: nowISO(),
        Clock:     incClockBeforeSend(),
    }

    if ch == "" {
        resp.Data["status"] = "erro"
        resp.Data["message"] = "nome inválido"
        return resp
    }

    channelsMu.Lock()
    for _, c := range channels {
        if c == ch {
            channelsMu.Unlock()
            resp.Data["status"] = "erro"
            resp.Data["message"] = "canal já existe"
            return resp
        }
    }
    channels = append(channels, ch)
    channelsMu.Unlock()

    go persistChannels()

    le := LogEntry{
        ID:        uuid.NewString(),
        Type:      "create_channel",
        Data:      map[string]interface{}{"channel": ch},
        Timestamp: nowISO(),
        Clock:     env.Clock,
    }

    logsMu.Lock()
    logs = append(logs, le)
    logsMu.Unlock()

    go persistLogs()
    publishReplicationEvent(le)

    resp.Data["status"] = "sucesso"
    resp.Data["channel"] = ch
    return resp
}

// ---------------------------------------------
// LIST CHANNELS
// ---------------------------------------------
func handleListChannels(env Envelope) Envelope {
    channelsMu.Lock()
    cpy := make([]string, len(channels))
    copy(cpy, channels)
    channelsMu.Unlock()

    return Envelope{
        Service:   "channels",
        Data:      map[string]interface{}{"channels": cpy},
        Timestamp: nowISO(),
        Clock:     incClockBeforeSend(),
    }
}

// ---------------------------------------------
// LOGIN
// ---------------------------------------------
func handleLogin(env Envelope) Envelope {
    user := ""
    if v, ok := env.Data["user"].(string); ok {
        user = normalize(v)
    }

    resp := Envelope{
        Service:   "login",
        Data:      map[string]interface{}{},
        Timestamp: nowISO(),
        Clock:     incClockBeforeSend(),
    }

    if user == "" {
        resp.Data["status"] = "erro"
        resp.Data["description"] = "usuário inválido"
        return resp
    }

    usersMu.Lock()
    found := false
    for _, u := range users {
        if u == user {
            found = true
            break
        }
    }
    if !found {
        users = append(users, user)
    }
    usersMu.Unlock()

    go persistUsers()

    resp.Data["status"] = "sucesso"
    resp.Data["user"] = user
    return resp
}

// ---------------------------------------------
// PUBLISH (canal público)
// ---------------------------------------------
func handlePublish(env Envelope) Envelope {
    channel := ""
    if v, ok := env.Data["channel"].(string); ok {
        channel = normalize(v)
    }

    user := ""
    if v, ok := env.Data["user"].(string); ok {
        user = normalize(v)
    }

    msg := ""
    if v, ok := env.Data["message"].(string); ok {
        msg = v
    }

    resp := Envelope{
        Service:   "publish",
        Data:      map[string]interface{}{},
        Timestamp: nowISO(),
        Clock:     incClockBeforeSend(),
    }

    // valida canal
    channelsMu.Lock()
    okChan := false
    for _, c := range channels {
        if c == channel {
            okChan = true
            break
        }
    }
    channelsMu.Unlock()

    if !okChan {
        resp.Data["status"] = "erro"
        resp.Data["message"] = "canal inexistente"
        return resp
    }

    // LOG
    le := LogEntry{
        ID:        uuid.NewString(),
        Type:      "publish",
        Data:      map[string]interface{}{"channel": channel, "user": user, "message": msg},
        Timestamp: nowISO(),
        Clock:     env.Clock,
    }

    logsMu.Lock()
    logs = append(logs, le)
    logsMu.Unlock()

    go persistLogs()
    publishReplicationEvent(le)

    // PUBLICAÇÃO REAL (tópico = canal)
    payload := msgpackMarshal(Envelope{
        Service:   "publish",
        Timestamp: nowISO(),
        Clock:     incClockBeforeSend(),
        Data: map[string]interface{}{
            "channel": channel,
            "user":    user,
            "message": msg,
        },
    })

    pubSocketMu.Lock()
    if pubSocket != nil {
        pubSocket.SendMessage(channel, payload)
    }
    pubSocketMu.Unlock()

    // resposta ao cliente
    resp.Data["status"] = "sucesso"
    return resp
}

// ---------------------------------------------
// PRIVATE MESSAGE (DIRECT MESSAGE)
// ---------------------------------------------
func handleMessage(env Envelope) Envelope {
    dst := ""
    if v, ok := env.Data["dst"].(string); ok {
        dst = normalize(v)
    }

    src := ""
    if v, ok := env.Data["src"].(string); ok {
        src = normalize(v)
    }

    msg := ""
    if v, ok := env.Data["message"].(string); ok {
        msg = v
    }

    resp := Envelope{
        Service:   "message",
        Data:      map[string]interface{}{},
        Timestamp: nowISO(),
        Clock:     incClockBeforeSend(),
    }

    // valida destino
    usersMu.Lock()
    exists := false
    for _, u := range users {
        if u == dst {
            exists = true
        }
    }
    usersMu.Unlock()

    if !exists {
        resp.Data["status"] = "erro"
        resp.Data["message"] = "usuário destino não existe"
        return resp
    }

    // LOG
    le := LogEntry{
        ID:        uuid.NewString(),
        Type:      "private",
        Data:      map[string]interface{}{"src": src, "dst": dst, "message": msg},
        Timestamp: nowISO(),
        Clock:     env.Clock,
    }

    logsMu.Lock()
    logs = append(logs, le)
    logsMu.Unlock()

    go persistLogs()
    publishReplicationEvent(le)

    // PUBLICAÇÃO PARA O DESTINO (tópico = usuário)
    payload := msgpackMarshal(Envelope{
        Service:   "message",
        Timestamp: nowISO(),
        Clock:     incClockBeforeSend(),
        Data: map[string]interface{}{
            "src":     src,
            "dst":     dst,
            "message": msg,
        },
    })

    pubSocketMu.Lock()
    if pubSocket != nil {
        pubSocket.SendMessage(dst, payload)
    }
    pubSocketMu.Unlock()

    resp.Data["status"] = "sucesso"
    return resp
}

// ---------------------------------------------
// SUBSCRIBE / UNSUBSCRIBE
// ---------------------------------------------
func handleSubscribe(env Envelope) Envelope {
    user := ""
    if v, ok := env.Data["user"].(string); ok {
        user = normalize(v)
    }

    topic := ""
    if v, ok := env.Data["topic"].(string); ok {
        topic = normalize(v)
    }

    resp := Envelope{
        Service:   "subscribe",
        Data:      map[string]interface{}{},
        Timestamp: nowISO(),
        Clock:     incClockBeforeSend(),
    }

    subMu.Lock()
    arr := subscriptions[user]
    found := false
    for _, s := range arr {
        if s == topic {
            found = true
        }
    }
    if !found {
        subscriptions[user] = append(arr, topic)
        go persistSubs()
    }
    subMu.Unlock()

    resp.Data["status"] = "sucesso"
    return resp
}

func handleUnsubscribe(env Envelope) Envelope {
    user := ""
    if v, ok := env.Data["user"].(string); ok {
        user = normalize(v)
    }

    topic := ""
    if v, ok := env.Data["topic"].(string); ok {
        topic = normalize(v)
    }

    resp := Envelope{
        Service:   "unsubscribe",
        Data:      map[string]interface{}{},
        Timestamp: nowISO(),
        Clock:     incClockBeforeSend(),
    }

    subMu.Lock()
    arr := subscriptions[user]
    newArr := []string{}
    for _, s := range arr {
        if s != topic {
            newArr = append(newArr, s)
        }
    }
    subscriptions[user] = newArr
    go persistSubs()
    subMu.Unlock()

    resp.Data["status"] = "sucesso"
    return resp
}
