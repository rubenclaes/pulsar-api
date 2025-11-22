package api

type EventRequest struct {
	EventType    string                 `json:"eventType" binding:"required"`
	SourceSystem string                 `json:"sourceSystem" binding:"required"`
	Topic        string                 `json:"topic,omitempty"` // OVERRIDE
	Payload      map[string]interface{} `json:"payload" binding:"required"`
}

type BatchRequest struct {
	Events  []EventRequest `json:"events" binding:"required"`
	DelayMs int            `json:"delayMs,omitempty"` // delay tussen events
	Rate    int            `json:"rate,omitempty"`    // msgs/sec, alternatief voor DelayMs
	// Later kun je nog numMessages, randomize, etc. toevoegen om pulsar-perf te emuleren
}
