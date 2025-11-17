package main

import (
    "encoding/json"
    "os"
)

func loadJSON(path string, dest interface{}) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()
    return json.NewDecoder(f).Decode(dest)
}

func persistUsers() {
    saveJSON(usersFile, users)
}

func persistChannels() {
    saveJSON(channelsFile, channels)
}

func persistSubs() {
    saveJSON(subsFile, subscriptions)
}

func persistLogs() {
    saveJSON(logsFile, logs)
}

func saveJSON(path string, data interface{}) {
    f, _ := os.Create(path)
    defer f.Close()
    enc := json.NewEncoder(f)
    enc.SetIndent("", "  ")
    enc.Encode(data)
}
