package file

import (
	"context"
	"io"
	"uuid"
)

type Repository interface {
	Upload(ctx context.Context, key string, src io.Reader) error
	Get(ctx context.Context, key string, dst io.Writer) error
}

type Service struct {
	r Repository
}

func NewService(r Repository) *Service {
	return &Service{r: r}
}

func (s *Service) Save(ctx context.Context, id uuid.UUID, src io.Reader) error {
	return s.r.Upload(ctx, id.String(), src)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID, dst io.Writer) error {
	return s.r.Get(ctx, id.String(), dst)
}
