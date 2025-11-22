package pulsar

import (
	"context"
	"fmt"

	"github.com/apache/pulsar-client-go/pulsar"
	"go.uber.org/zap"

	customauth "github.com/rubenclaes/pulsar-api/internal/auth"
)

type Producer struct {
	client       pulsar.Client
	defaultTopic string
	log          *zap.Logger
	tokenProv    *customauth.TokenProvider
}

type Config struct {
	URL          string
	DefaultTopic string
	Auth         customauth.OAuthConfig
}

func NewProducer(log *zap.Logger, cfg Config, tokenProv *customauth.TokenProvider) (*Producer, error) {
	var auth pulsar.Authentication

	if cfg.Auth.Enabled {
		if tokenProv == nil {
			return nil, fmt.Errorf("auth enabled but token provider is nil")
		}
		// Initieel token ophalen
		token, err := tokenProv.GetToken(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to fetch initial token: %w", err)
		}
		auth = pulsar.NewAuthenticationToken(token)
	}

	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL:            cfg.URL,
		Authentication: auth,
	})
	if err != nil {
		return nil, err
	}

	return &Producer{
		client:       client,
		defaultTopic: cfg.DefaultTopic,
		log:          log,
		tokenProv:    tokenProv,
	}, nil
}

func (p *Producer) Close() {
	p.client.Close()
}

// Send stuurt een message naar topic (of defaultTopic als leeg).
func (p *Producer) Send(ctx context.Context, topic string, payload []byte) error {
	if topic == "" {
		topic = p.defaultTopic
	}

	prod, err := p.client.CreateProducer(pulsar.ProducerOptions{
		Topic: topic,
	})
	if err != nil {
		return err
	}
	defer prod.Close()

	_, err = prod.Send(ctx, &pulsar.ProducerMessage{
		Payload: payload,
	})
	if err != nil {
		return err
	}
	return nil
}
