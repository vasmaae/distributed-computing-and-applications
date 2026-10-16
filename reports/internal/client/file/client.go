package file

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
	"uuid"
)

type Client struct {
	URL string
}

func NewClient(url string) *Client {
	return &Client{URL: url}
}

func (c *Client) Save(ctx context.Context, id uuid.UUID, src io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.URL+"/api/v1/files/"+id.String(), src)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/csv")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("file service returned status %d", resp.StatusCode)
	}
	return nil
}
