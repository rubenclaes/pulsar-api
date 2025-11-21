package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/rubenclaes/pulsar-api/internal/api"
	"github.com/rubenclaes/pulsar-api/internal/logging"
	"github.com/rubenclaes/pulsar-api/internal/middleware"
	"github.com/rubenclaes/pulsar-api/internal/pulsar"
)

func main() {
	logging.Init()
	defer logging.Sync()
	log := logging.Logger

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	// 1. Local dev path
	v.AddConfigPath("./config") // go run
	v.AddConfigPath(".")        // fallback

	// 2. Standard OS config locations (cross-platform best practice)
	v.AddConfigPath("/etc/pulsar-api/")
	v.AddConfigPath("$HOME/.config/pulsar-api/")
	v.AddConfigPath("$XDG_CONFIG_HOME/pulsar-api/")

	// Load config
	if err := v.ReadInConfig(); err != nil {
		log.Fatal("Failed to load config.yaml", zap.Error(err))
	}

	brokerURL := v.GetString("pulsar.url")
	topic := v.GetString("pulsar.defaultTopic")
	dryRun := v.GetBool("api.dryRun")
	port := v.GetInt("api.port")
	schemaMap := v.GetStringMapString("schemas")

	if brokerURL == "" || topic == "" {
		log.Fatal("PULSAR_URL and PULSAR_TOPIC must be set")
	}

	var producer *pulsar.Producer
	if !dryRun {
		producer = pulsar.NewProducer(brokerURL, topic)
		defer producer.Close()
	}

	handler := api.NewEventHandler(log, producer, topic, dryRun, schemaMap)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(middleware.CorrelationID())

	// HEALTH
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// OPENAPI
	r.GET("/openapi.yaml", func(c *gin.Context) {
		c.Header("Content-Type", "application/yaml")
		c.String(http.StatusOK, openAPISpec)
	})

	// ----------------------------------------
	// UI — ONLY ON /ui  (NO REDIRECTS ANYWHERE)
	// ----------------------------------------
	r.GET("/ui", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, uiHTML)
	})

	// ----------------------------------------
	// API
	// ----------------------------------------
	v1 := r.Group("/api/v1")
	{
		v1.POST("/events", handler.PostEvent)
		v1.POST("/events/batch", handler.PostBatch)
	}

	// START SERVER
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Info("Starting API", zap.String("address", addr))
	r.Run(addr)
}

const uiHTML = `
<!DOCTYPE html>
<html lang="nl">
<head>
  <meta charset="UTF-8" />
  <title>Acerta Event Tester</title>

  <!-- Tailwind CDN -->
  <script src="https://cdn.tailwindcss.com"></script>

  <!-- Tailwind config for Acerta colors -->
  <script>
    tailwind.config = {
      theme: {
        extend: {
          colors: {
            acertaBlue: '#003366',
            acertaCyan: '#00a9c7'
          }
        }
      }
    }
  </script>

  <!-- ShadCN UI base styles -->
  <link
    rel="stylesheet"
    href="https://unpkg.com/@shadcn/ui/styles.css"
  />
</head>

<body class="bg-gray-100 text-acertaBlue">

  <!-- HEADER -->
  <header class="bg-acertaBlue text-white px-6 py-4 shadow">
    <h1 class="text-xl font-semibold">Acerta Pulsar Event Tester</h1>
  </header>

  <!-- MAIN WRAPPER -->
  <main class="max-w-4xl mx-auto p-6">

    <div class="bg-white shadow-md rounded-lg p-6 space-y-6">

      <!-- ENDPOINT -->
      <div class="space-y-2">
        <label class="font-medium">Endpoint</label>
        <div class="flex gap-2">
          <input id="endpoint"
                 class="border rounded px-3 py-2 flex-1"
                 value="/api/v1/events" />
          <button onclick="setSingle()"
                  class="bg-acertaBlue text-white px-3 py-2 rounded">
            Single
          </button>
          <button onclick="setBatch()"
                  class="bg-acertaCyan text-white px-3 py-2 rounded">
            Batch
          </button>
        </div>
      </div>

      <!-- BODY -->
      <div class="space-y-2">
        <label class="font-medium">Body (JSON)</label>
        <textarea id="body"
                  class="border rounded w-full h-64 font-mono p-3"></textarea>
      </div>

      <!-- SEND BUTTON -->
      <div class="flex justify-end">
        <button onclick="send()"
                class="bg-acertaCyan text-white px-5 py-2 rounded text-lg">
          Versturen
        </button>
      </div>

      <!-- RESPONSE -->
      <div>
        <h2 class="font-semibold text-lg mb-2">Response</h2>
        <pre id="response"
             class="bg-black text-green-400 p-4 rounded overflow-auto h-64"></pre>
      </div>

    </div>
  </main>

  <!-- JS -->
  <script>
    function setSingle() {
      document.getElementById("endpoint").value = "/api/v1/events";
      document.getElementById("body").value = JSON.stringify(
        {
          eventType: "SIGNALITIEK_ERROR",
          sourceSystem: "EverESSt",
          payload: {
            errorCode: "999999",
            message: "Test event",
            employerId: "123456"
          }
        },
        null,
        2
      );
    }

    function setBatch() {
      document.getElementById("endpoint").value = "/api/v1/events/batch";
      document.getElementById("body").value = JSON.stringify(
        [
          {
            eventType: "SIGNALITIEK_ERROR",
            sourceSystem: "EverESSt",
            payload: {
              errorCode: "999999",
              message: "Test batch 1",
              employerId: "123456"
            }
          },
          {
            eventType: "WAGE_ERROR",
            sourceSystem: "EverESSt",
            payload: {
              dossierId: "ABC-123"
            }
          }
        ],
        null,
        2
      );
    }

    async function send() {
      const ep = document.getElementById("endpoint").value;
      const body = document.getElementById("body").value;
      const resp = document.getElementById("response");

      resp.textContent = "⏳ Versturen...";

      try {
        const res = await fetch(ep, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body
        });

        const text = await res.text();

        try {
          resp.textContent = JSON.stringify(JSON.parse(text), null, 2);
        } catch {
          resp.textContent = text;
        }
      } catch (e) {
        resp.textContent = "❌ Error: " + e;
      }
    }

    setSingle();
  </script>
</body>
</html>
`

const openAPISpec = `
openapi: 3.0.3
info:
  title: Pulsar Event API
  version: 1.0.0
paths:
  /api/v1/events:
    post:
      summary: Send a single event to Pulsar
      operationId: postEvent
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/EventRequest'
      responses:
        "201":
          description: Event sent
  /api/v1/events/batch:
    post:
      summary: Send multiple events in one call
      operationId: postEventBatch
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: array
              items:
                $ref: '#/components/schemas/EventRequest'
      responses:
        "200":
          description: Batch result
components:
  schemas:
    EventRequest:
      type: object
      required:
        - eventType
        - sourceSystem
        - payload
      properties:
        eventType:
          type: string
        sourceSystem:
          type: string
        payload:
          type: object
`
