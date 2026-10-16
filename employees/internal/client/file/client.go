package file

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"uuid"

	"github.com/vasmaae/distributed-computing-and-applications/common/telemetry"
	errs "github.com/vasmaae/distributed-computing-and-applications/employees/internal/errors"
)

type Client struct {
	URL string
}

type Result struct {
	Data []byte
	Err  error
}

func NewClient(url string) *Client {
	return &Client{URL: url}
}

func (c *Client) GetAsync(ctx context.Context, id uuid.UUID) <-chan Result {
	result := make(chan Result, 1)
	go func() {
		defer close(result)
		data, err := c.get(ctx, id)
		result <- Result{Data: data, Err: err}
	}()
	return result
}

func (c *Client) get(ctx context.Context, id uuid.UUID) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL+"/api/v1/files/"+id.String(), nil)
	if err != nil {
		return nil, err
	}
	client := telemetry.HTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode == http.StatusNotFound {
		return nil, errs.ErrFileNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("file service returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(body) > 8<<20 {
		return nil, fmt.Errorf("CSV exceeds 8 MiB")
	}
	return body, nil
}
