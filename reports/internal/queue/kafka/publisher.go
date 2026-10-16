package kafka

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
	"uuid"

	"github.com/segmentio/kafka-go"
)

type Publisher struct {
	writer *kafka.Writer
}

func NewPublisher(brokers, topic string) *Publisher {
	return &Publisher{writer: &kafka.Writer{
		Addr: kafka.TCP(strings.Split(brokers, ",")...), Topic: topic,
		Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll,
		BatchBytes: 9 << 20, BatchTimeout: 10 * time.Millisecond,
		ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, MaxAttempts: 3,
	}}
}

func (p *Publisher) Close() error {
	return p.writer.Close()
}

func (p *Publisher) Save(ctx context.Context, id uuid.UUID, src io.Reader) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	body, err := io.ReadAll(io.LimitReader(src, (8<<20)+1))
	if err != nil {
		return err
	}
	if len(body) == 0 || len(body) > 8<<20 {
		return fmt.Errorf("CSV must be between 1 byte and 8 MiB")
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key: []byte(id.String()), Value: body,
		Headers: []kafka.Header{{Key: "content-type", Value: []byte("text/csv")}},
	})
}
