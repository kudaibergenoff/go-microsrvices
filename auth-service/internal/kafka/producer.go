package kafka

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
)

var (
	kafkaMessagesProduced = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_messages_produced_total",
		Help: "Total number of Kafka messages produced",
	}, []string{"topic", "status"})
)

type UserRegisteredEvent struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers, topic string) *Producer {
	w := &kafka.Writer{
		Addr:                   kafka.TCP(strings.Split(brokers, ",")...),
		Topic:                  topic,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}
	return &Producer{writer: w}
}

func (p *Producer) PublishUserRegistered(ctx context.Context, event UserRegisteredEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Inject trace context into Kafka headers
	headers := injectTraceContext(ctx)

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:     []byte(event.Email),
		Value:   data,
		Headers: headers,
	})
	if err != nil {
		kafkaMessagesProduced.WithLabelValues(p.writer.Topic, "error").Inc()
		log.Printf("failed to publish user.registered event: %v", err)
		return err
	}

	kafkaMessagesProduced.WithLabelValues(p.writer.Topic, "success").Inc()
	log.Printf("published user.registered event for user_id=%d email=%s", event.UserID, event.Email)
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

// kafkaHeaderCarrier adapts Kafka headers to OTel propagation.
type kafkaHeaderCarrier []kafka.Header

func (c *kafkaHeaderCarrier) Get(key string) string {
	for _, h := range *c {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *kafkaHeaderCarrier) Set(key, value string) {
	for i, h := range *c {
		if h.Key == key {
			(*c)[i].Value = []byte(value)
			return
		}
	}
	*c = append(*c, kafka.Header{Key: key, Value: []byte(value)})
}

func (c *kafkaHeaderCarrier) Keys() []string {
	keys := make([]string, len(*c))
	for i, h := range *c {
		keys[i] = h.Key
	}
	return keys
}

func injectTraceContext(ctx context.Context) []kafka.Header {
	carrier := kafkaHeaderCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, &carrier)
	return carrier
}
