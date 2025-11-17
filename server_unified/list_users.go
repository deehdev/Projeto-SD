package main

func handleListUsers(env Envelope) Envelope {
    usersMu.Lock()
    arr := make([]string, len(users))
    copy(arr, users)
    usersMu.Unlock()

    return Envelope{
        Service:   "users",
        Timestamp: nowISO(),
        Clock:     incClockBeforeSend(),
        Data: map[string]interface{}{
            "users": arr,
        },
    }
}
