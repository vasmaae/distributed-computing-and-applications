package kafka

import (
	"context"
	"errors"
	"io"
	"testing"
	"uuid"

	"github.com/segmentio/kafka-go"
)

type reader struct {
	msg                kafka.Message
	fetched, committed bool
}

func (r *reader) FetchMessage(context.Context) (kafka.Message, error) {
	if r.fetched {
		return kafka.Message{}, context.Canceled
	}
	r.fetched = true
	return r.msg, nil
}

func (r *reader) CommitMessages(context.Context, ...kafka.Message) error {
	r.committed = true
	return nil
}

type storage struct {
	err  error
	id   uuid.UUID
	data string
}

func (s *storage) Save(_ context.Context, id uuid.UUID, src io.Reader) error {
	body, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	s.id, s.data = id, string(body)
	return s.err
}

func TestCommitOnlyAfterSavingReport(t *testing.T) {
	id := uuid.New()
	for _, fail := range []bool{false, true} {
		s := &storage{}
		if fail {
			s.err = errors.New("S3 unavailable")
		}
		r := &reader{msg: kafka.Message{Key: []byte(id.String()), Value: []byte("id,fio\n"), Headers: []kafka.Header{{Key: "content-type", Value: []byte("text/csv")}}}}
		err := consume(t.Context(), r, s)
		if r.committed == fail || (fail && !errors.Is(err, s.err)) || (!fail && !errors.Is(err, context.Canceled)) {
			t.Fatalf("fail=%v committed=%v error=%v", fail, r.committed, err)
		}
		if s.id != id || s.data != "id,fio\n" {
			t.Fatal("message changed in transit")
		}
	}
	r := &reader{msg: kafka.Message{Key: []byte("invalid")}}
	if err := consume(t.Context(), r, &storage{}); !errors.Is(err, context.Canceled) || !r.committed {
		t.Fatal("invalid message blocks the partition")
	}
}
