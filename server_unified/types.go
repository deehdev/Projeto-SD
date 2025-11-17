package main

// Envelope é o formato padrão de troca de mensagens
type Envelope struct {
	Service   string                 `json:"service"`
	Data      map[string]interface{} `json:"data"`
	Timestamp string                 `json:"timestamp"`
	Clock     int                    `json:"clock"`
}

// LogEntry representa eventos para replicação/persistência
type LogEntry struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp string                 `json:"timestamp"`
	Clock     int                    `json:"clock"`
}
