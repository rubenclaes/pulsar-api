package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const CorrelationIDHeader = "X-Correlation-ID"
const correlationKey = "correlationId"

func CorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		corrID := c.GetHeader(CorrelationIDHeader)
		if corrID == "" {
			corrID = uuid.NewString()
		}

		// in context steken
		c.Set(correlationKey, corrID)
		// ook terug in response header
		c.Writer.Header().Set(CorrelationIDHeader, corrID)

		c.Next()
	}
}

func GetCorrelationID(c *gin.Context) string {
	if v, ok := c.Get(correlationKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
