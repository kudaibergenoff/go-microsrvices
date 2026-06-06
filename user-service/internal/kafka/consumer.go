package kafka

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/zhanuzak/microservices/user-service/internal/model"
	"github.com/zhanuzak/microservices/user-service/internal/repository"
)

var (
	kafkaMessagesConsumed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_messages_consumed_total",
		Help: "Total number of Kafka messages consumed",
	}, []string{"topic", "status"})

	kafkaConsumerLag = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kafka_consumer_lag",
		Help: "Kafka consumer lag (offset difference)",
	}, []string{"topic", "partition"})

	kafkaProcessDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kafka_message_process_duration_seconds",
		Help:    "Duration of Kafka message processing",
		Buckets: prometheus.DefBuckets,
	}, []string{"topic"})
)

var tracer = otel.Tracer("kafka-consumer")

type UserRegisteredEvent struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

type Consumer struct {
	reader *kafka.Reader
	repo   repository.UserRepository
}

func NewConsumer(brokers, topic, groupID string, repo repository.UserRepository) *Consumer {
	brokerList := strings.Split(brokers, ",")
	ensureTopic(brokerList[0], topic)

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokerList,
		Topic:          topic,
		GroupID:        groupID,
		StartOffset:    kafka.FirstOffset,
		CommitInterval: time.Second,
	})
	return &Consumer{reader: r, repo: repo}
}

func ensureTopic(broker, topic string) {
	for i := 0; i < 10; i++ {
		conn, err := kafka.DialLeader(context.Background(), "tcp", broker, topic, 0)
		if err != nil {
			log.Printf("waiting for kafka topic %s (attempt %d): %v", topic, i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		conn.Close()
		log.Printf("kafka topic %s is ready", topic)
		return
	}

	conn, err := net.DialTimeout("tcp", broker, 5*time.Second)
	if err != nil {
		log.Printf("cannot reach kafka broker: %v", err)
		return
	}
	conn.Close()

	controller, err := kafka.Dial("tcp", broker)
	if err != nil {
		log.Printf("cannot dial kafka: %v", err)
		return
	}
	defer controller.Close()

	err = controller.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	if err != nil {
		log.Printf("failed to create topic %s: %v", topic, err)
	} else {
		log.Printf("created kafka topic %s", topic)
	}
}

func (c *Consumer) Start(ctx context.Context) {
	log.Printf("kafka consumer started, listening on topic: %s", c.reader.Config().Topic)
	topic := c.reader.Config().Topic

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("kafka consumer stopped")
				return
			}
			log.Printf("kafka read error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		start := time.Now()

		// Extract trace context from Kafka headers
		parentCtx := extractTraceContext(ctx, msg.Headers)
		spanCtx, span := tracer.Start(parentCtx, "kafka.consume "+topic,
			trace.WithAttributes(
				attribute.String("messaging.system", "kafka"),
				attribute.String("messaging.destination", topic),
				attribute.Int64("messaging.kafka.partition", int64(msg.Partition)),
				attribute.Int64("messaging.kafka.offset", msg.Offset),
			),
		)

		// Update consumer lag metric
		stats := c.reader.Stats()
		kafkaConsumerLag.WithLabelValues(topic, "0").Set(float64(stats.Lag))

		var event UserRegisteredEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("failed to unmarshal event: %v", err)
			kafkaMessagesConsumed.WithLabelValues(topic, "error").Inc()
			span.End()
			continue
		}

		log.Printf("received user.registered event: user_id=%d email=%s", event.UserID, event.Email)

		user := &model.User{
			ID:    event.UserID,
			Email: event.Email,
			Name:  event.Name,
		}

		if err := c.repo.Create(spanCtx, user); err != nil {
			log.Printf("failed to create user from event: %v", err)
			kafkaMessagesConsumed.WithLabelValues(topic, "error").Inc()
			span.End()
			continue
		}

		kafkaMessagesConsumed.WithLabelValues(topic, "success").Inc()
		kafkaProcessDuration.WithLabelValues(topic).Observe(time.Since(start).Seconds())
		span.End()

		log.Printf("user synced from auth: id=%d email=%s", event.UserID, event.Email)
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
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

func extractTraceContext(ctx context.Context, headers []kafka.Header) context.Context {
	carrier := kafkaHeaderCarrier(headers)
	return otel.GetTextMapPropagator().Extract(ctx, &carrier)
}
