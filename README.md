# Pulsar Event Tester – Handleiding

Deze tool laat toe om JSON-events naar Pulsar te sturen via een REST API of via een ingebouwde webpagina.

## Bestandsstructuur

Download of ontzip de map:

```
pulsar-api/
  pulsar-api.exe
  config/
    config.yaml
  schemas/
    *.json
```

## Runnen 
```golang
go run ./cmd/api
```

## Applicatie starten

Windows (PowerShell):

```powershell
.\pulsar-api.exe
```

Mac/Linux:

```bash
./pulsar-api
```

De API draait standaard op:
[http://localhost:8080](http://localhost:8080)

## Webinterface openen

[http://localhost:8080/ui](http://localhost:8080/ui)

Hier kan je eenvoudig JSON events versturen zonder Postman of andere tools.

## Een event versturen via REST

POST naar:

```
http://localhost:8080/api/v1/events
```

Headers:

```
Content-Type: application/json
X-Correlation-ID: optioneel
```

Voorbeeld body:

```json
{
  "eventType": "SIGNALITIEK_ERROR",
  "sourceSystem": "EverESSt",
  "payload": {
    "errorCode": "999999",
    "message": "Test event",
    "employerId": "123456"
  }
}
```

De API valideert dit event tegen de juiste JSON schema (gebaseerd op AsyncAPI).

## Batch van events versturen

POST naar:

```
http://localhost:8080/api/v1/events/batch
```

Voorbeeld:

```json
[
  {
    "eventType": "SIGNALITIEK_ERROR",
    "sourceSystem": "EverESSt",
    "payload": {
      "errorCode": "999999",
      "message": "Test batch 1",
      "employerId": "123456"
    }
  },
  {
    "eventType": "WAGE_ERROR",
    "sourceSystem": "EverESSt",
    "payload": {
      "dossierId": "ABC-123"
    }
  }
]
```

De API geeft per event terug of het valid, invalid, sent of dry-run was.

## Configuratie

Open:

```
config/config.yaml
```

Voorbeeld:

```yaml
pulsar:
  url: "pulsar://localhost:6650"
  defaultTopic: "persistent://tenant/ns/default-topic"

api:
  dryRun: true
  port: 8080

schemas:
  SIGNALITIEK_ERROR: "schemas/signalitiek_error.json"
  WAGE_ERROR: "schemas/wage_error.json"
```

Belangrijk:

* `dryRun: true` betekent dat events niet naar Pulsar gestuurd worden.
* `dryRun: false` stuurt wel echt naar Pulsar.
* Elk eventType heeft zijn eigen schema file.

## API Documentatie (OpenAPI)

De documentatie staat op:

```
http://localhost:8080/openapi.yaml
```

## Logs

Tijdens het draaien toont de applicatie:

* validatiefouten
* schema mismatches
* correlation IDs
* verstuurde events
* dry-run status

## Veelvoorkomende problemen

"Config not found": zorg dat `config/config.yaml` bestaat in dezelfde map als de executable.

"Schema file not found": controleer dat de schema-bestanden bestaan en de paden kloppen in `config.yaml`.

Executable crasht bij dubbelklikken: start via PowerShell:

```powershell
.\pulsar-api.exe
```