package kafka

import (
	"context"
	"encoding/json"
	"log"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/zhanuzak/microservices/order-service/internal/model"
)

type Producer struct {
	writer *kafkago.Writer
}

func NewProducer(brokers, topic string) *Producer {
	writer := &kafkago.Writer{
		Addr:     kafkago.TCP(brokers),
		Topic:    topic,
		Balancer: &kafkago.LeastBytes{},
	}
	return &Producer{writer: writer}
}

func (p *Producer) PublishOrderEvent(ctx context.Context, event model.OrderEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = p.writer.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(json.Number(string(rune(event.OrderID)))),
		Value: data,
	})
	if err != nil {
		log.Printf("ERROR: failed to publish order event: %v", err)
		return err
	}

	log.Printf("Published order.%s event: order_id=%d", event.Status, event.OrderID)
	return nil
}

func (p *Producer) Close() {
	p.writer.Close()
}
