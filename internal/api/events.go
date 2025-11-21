package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rubenclaes/pulsar-api/internal/middleware"
	"github.com/rubenclaes/pulsar-api/internal/pulsar"
)

type EventRequest struct {
	EventType    string                 `json:"eventType" binding:"required"`
	SourceSystem string                 `json:"sourceSystem" binding:"required"`
	Payload      map[string]interface{} `json:"payload" binding:"required"`
}

type EventResponse struct {
	Status        string        `json:"status"`
	Topic         string        `json:"topic"`
	Bytes         int           `json:"bytes"`
	DryRun        bool          `json:"dryRun"`
	CorrelationID string        `json:"correlationId"`
	MessageID     string        `json:"messageId,omitempty"`
	Event         *EventRequest `json:"event,omitempty"`
}

type BatchItemResult struct {
	Index         int           `json:"index"`
	Status        string        `json:"status"`
	Topic         string        `json:"topic,omitempty"`
	Bytes         int           `json:"bytes,omitempty"`
	MessageID     string        `json:"messageId,omitempty"`
	Error         string        `json:"error,omitempty"`
	CorrelationID string        `json:"correlationId"`
	Event         *EventRequest `json:"event,omitempty"`
}

type BatchResponse struct {
	Status  string            `json:"status"`
	Count   int               `json:"count"`
	DryRun  bool              `json:"dryRun"`
	Results []BatchItemResult `json:"results"`
}

// simpele mapping eventType -> Pulsar topic
var eventTypeTopicMap = map[string]string{
	"SIGNALITIEK_ERROR": "persistent://tenant/ns/signalitiek-errors",
	"WAGE_ERROR":        "persistent://tenant/ns/wage-errors",
	// default: valt terug op main topic uit env
}

// super simpele “schema”-checks per eventType
func validateEventSchema(req EventRequest) error {
	switch req.EventType {
	case "SIGNALITIEK_ERROR":
		if _, ok := req.Payload["errorCode"]; !ok {
			return errors.New("payload.errorCode is required for SIGNALITIEK_ERROR")
		}
		if _, ok := req.Payload["employerId"]; !ok {
			return errors.New("payload.employerId is required for SIGNALITIEK_ERROR")
		}
	case "WAGE_ERROR":
		if _, ok := req.Payload["dossierId"]; !ok {
			return errors.New("payload.dossierId is required for WAGE_ERROR")
		}
	}
	return nil
}

type EventHandler struct {
	Logger    *zap.Logger
	Producer  *pulsar.Producer
	Topic     string // default topic
	SchemaMap map[string]string
	DryRun    bool
}

func NewEventHandler(logger *zap.Logger, producer *pulsar.Producer, topic string, dryRun bool, schemaMap map[string]string) *EventHandler {
	return &EventHandler{
		Logger:    logger,
		Producer:  producer,
		Topic:     topic,
		DryRun:    dryRun,
		SchemaMap: schemaMap,
	}
}

func (h *EventHandler) resolveTopic(req EventRequest) string {
	if t, ok := eventTypeTopicMap[req.EventType]; ok {
		return t
	}
	// fallback naar default topic
	return h.Topic
}

// POST /api/v1/events
func (h *EventHandler) PostEvent(c *gin.Context) {
	log := h.Logger.With(
		zap.String("path", c.FullPath()),
		zap.String("method", c.Request.Method),
	)
	corrID := middleware.GetCorrelationID(c)

	var req EventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn("invalid request body", zap.Error(err), zap.String("correlationId", corrID))
		c.JSON(http.StatusBadRequest, gin.H{
			"status":        "error",
			"error":         "invalid request body",
			"details":       err.Error(),
			"correlationId": corrID,
		})
		return
	}

	if err := validateEventSchema(req); err != nil {
		log.Warn("schema validation failed",
			zap.Error(err),
			zap.String("eventType", req.EventType),
			zap.String("correlationId", corrID),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":        "error",
			"error":         "schema validation failed",
			"details":       err.Error(),
			"correlationId": corrID,
		})
		return
	}

	payloadBytes, err := json.Marshal(req)
	if err != nil {
		log.Error("failed to marshal payload", zap.Error(err), zap.String("correlationId", corrID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":        "error",
			"error":         "internal serialization error",
			"correlationId": corrID,
		})
		return
	}

	topic := h.resolveTopic(req)

	log.Info("Received event",
		zap.String("eventType", req.EventType),
		zap.String("sourceSystem", req.SourceSystem),
		zap.String("topic", topic),
		zap.Int("bytes", len(payloadBytes)),
		zap.String("correlationId", corrID),
	)

	resp := EventResponse{
		Topic:         topic,
		Bytes:         len(payloadBytes),
		DryRun:        h.DryRun,
		CorrelationID: corrID,
		Event:         &req,
	}

	if h.DryRun {
		log.Info("DRY-RUN → not sending to Pulsar", zap.String("correlationId", corrID))
		resp.Status = "dry-run"
		c.JSON(http.StatusOK, resp)
		return
	}

	msgID, err := h.Producer.Send(payloadBytes)
	if err != nil {
		log.Error("failed sending to Pulsar",
			zap.Error(err),
			zap.String("correlationId", corrID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":        "error",
			"error":         "failed sending to Pulsar",
			"details":       err.Error(),
			"correlationId": corrID,
		})
		return
	}

	resp.Status = "sent"
	resp.MessageID = msgID

	log.Info("Event sent to Pulsar",
		zap.String("messageId", msgID),
		zap.String("topic", topic),
		zap.String("correlationId", corrID),
	)

	c.JSON(http.StatusCreated, resp)
}

// POST /api/v1/events/batch
func (h *EventHandler) PostBatch(c *gin.Context) {
	log := h.Logger.With(
		zap.String("path", c.FullPath()),
		zap.String("method", c.Request.Method),
	)
	corrID := middleware.GetCorrelationID(c)

	var reqs []EventRequest
	if err := c.ShouldBindJSON(&reqs); err != nil {
		log.Warn("invalid batch body", zap.Error(err), zap.String("correlationId", corrID))
		c.JSON(http.StatusBadRequest, gin.H{
			"status":        "error",
			"error":         "invalid batch body",
			"details":       err.Error(),
			"correlationId": corrID,
		})
		return
	}

	results := make([]BatchItemResult, 0, len(reqs))

	for i, req := range reqs {
		itemCorr := corrID // je kan evt. per item een eigen ID genereren

		r := BatchItemResult{
			Index:         i,
			CorrelationID: itemCorr,
			Event:         &req,
		}

		if err := validateEventSchema(req); err != nil {
			r.Status = "error"
			r.Error = "schema validation failed: " + err.Error()
			results = append(results, r)
			continue
		}

		payloadBytes, err := json.Marshal(req)
		if err != nil {
			r.Status = "error"
			r.Error = "marshal error: " + err.Error()
			results = append(results, r)
			continue
		}

		topic := h.resolveTopic(req)
		r.Topic = topic
		r.Bytes = len(payloadBytes)

		if h.DryRun {
			r.Status = "dry-run"
			results = append(results, r)
			continue
		}

		msgID, err := h.Producer.Send(payloadBytes)
		if err != nil {
			r.Status = "error"
			r.Error = "send error: " + err.Error()
			results = append(results, r)
			continue
		}

		r.Status = "sent"
		r.MessageID = msgID
		results = append(results, r)
	}

	status := "sent"
	if h.DryRun {
		status = "dry-run"
	}

	resp := BatchResponse{
		Status:  status,
		Count:   len(results),
		DryRun:  h.DryRun,
		Results: results,
	}

	c.JSON(http.StatusOK, resp)
}
