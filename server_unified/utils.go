package main

import (
    "strings"
    "time"
)

func nowISO() string {
    return time.Now().UTC().Format(time.RFC3339Nano)
}

func normalize(s string) string {
    return strings.ToLower(strings.TrimSpace(s))
}

func incClockBeforeSend() int {
    logicalClock++
    return logicalClock
}

func updateClockOnRecv(c int) {
    if c > logicalClock {
        logicalClock = c
    }
    logicalClock++
}
