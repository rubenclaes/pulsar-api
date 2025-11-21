package pulsar

import (
	"context"
	"log"

	pulsargo "github.com/apache/pulsar-client-go/pulsar"
)

type Producer struct {
	client   pulsargo.Client
	producer pulsargo.Producer
}

func NewProducer(brokerURL, topic string) *Producer {
	client, err := pulsargo.NewClient(pulsargo.ClientOptions{
		URL: brokerURL,
	})
	if err != nil {
		log.Fatalf("failed to create pulsar client: %v", err)
	}

	producer, err := client.CreateProducer(pulsargo.ProducerOptions{
		Topic: topic,
	})
	if err != nil {
		log.Fatalf("failed to create pulsar producer: %v", err)
	}

	return &Producer{
		client:   client,
		producer: producer,
	}
}

// returns Pulsar message ID as string
func (p *Producer) Send(msg []byte) (string, error) {
	msgID, err := p.producer.Send(context.Background(), &pulsargo.ProducerMessage{
		Payload: msg,
	})
	if err != nil {
		return "", err
	}
	return msgID.String(), nil
}

func (p *Producer) Close() {
	p.producer.Close()
	p.client.Close()
}
