package kafka

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"
	"uuid"

	"github.com/segmentio/kafka-go"
)

type FileStorage interface {
	Save(ctx context.Context, id uuid.UUID, src io.Reader) error
}

type messageReader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
}

func Consume(ctx context.Context, brokers, topic, group string, files FileStorage) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(brokers, ","), Topic: topic, GroupID: group,
		MinBytes: 1, MaxBytes: 10 << 20, MaxWait: time.Second, QueueCapacity: 1,
		StartOffset: kafka.FirstOffset,
	})
	defer reader.Close() //nolint:errcheck
	return consume(ctx, reader, files)
}

func consume(ctx context.Context, reader messageReader, files FileStorage) error {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		if err := process(ctx, msg, files); err != nil {
			return err
		}
		if err := reader.CommitMessages(ctx, msg); err != nil {
			return fmt.Errorf("commit saved report offset: %w", err)
		}
	}
}

func process(ctx context.Context, msg kafka.Message, files FileStorage) error {
	id, err := uuid.Parse(string(msg.Key))
	contentType := ""
	for _, header := range msg.Headers {
		if header.Key == "content-type" {
			contentType = string(header.Value)
		}
	}
	if err != nil || id == uuid.Nil() || contentType != "text/csv" || len(msg.Value) == 0 || len(msg.Value) > 8<<20 {
		log.Print("skipped invalid report message")
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := files.Save(ctx, id, bytes.NewReader(msg.Value)); err != nil {
		return fmt.Errorf("save queued report: %w", err)
	}
	return nil
}
