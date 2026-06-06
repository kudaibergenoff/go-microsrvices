package kafka

import (
	"context"
	"encoding/json"
	"log"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/zhanuzak/microservices/notification-service/internal/model"
	"github.com/zhanuzak/microservices/notification-service/internal/smtp"
)

type Consumer struct {
	reader *kafkago.Reader
	mailer *smtp.Mailer
}

func NewConsumer(brokers, topic, groupID string, mailer *smtp.Mailer) *Consumer {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  []string{brokers},
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})

	return &Consumer{reader: reader, mailer: mailer}
}

func (c *Consumer) Start(ctx context.Context) {
	log.Printf("Kafka consumer started, listening on topic: %s", c.reader.Config().Topic)

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("Kafka consumer stopped")
				return
			}
			log.Printf("ERROR: failed to read message: %v", err)
			continue
		}

		var event model.UserRegisteredEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("ERROR: failed to unmarshal event: %v", err)
			continue
		}

		log.Printf("Received user.registered event: user_id=%d email=%s name=%s", event.UserID, event.Email, event.Name)

		if err := c.mailer.SendWelcome(event.Email, event.Name); err != nil {
			log.Printf("ERROR: failed to send welcome email to %s: %v", event.Email, err)
		} else {
			log.Printf("Welcome email sent to %s", event.Email)
		}
	}
}

func (c *Consumer) Close() {
	c.reader.Close()
}
