package report

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/client/dto"
	errs "github.com/vasmaae/distributed-computing-and-applications/employees/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/handler/report"
)

var _ report.Client = (*Client)(nil)

type Client struct {
	URL string
}

func NewClient(url string) *Client {
	return &Client{URL: url}
}

func (c *Client) GetEmployeesReport(ctx context.Context) (dto.ReportResponse, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/api/v1/reports/employees-report", c.URL),
		nil,
	)
	if err != nil {
		return dto.ReportResponse{}, errs.ErrReportsClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return dto.ReportResponse{}, errs.ErrReportsClient
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return dto.ReportResponse{}, errs.ErrReportsClient
	}

	var reportResponse dto.ReportResponse
	if err := json.NewDecoder(resp.Body).Decode(&reportResponse); err != nil {
		return dto.ReportResponse{}, errs.ErrReportsClient
	}

	return reportResponse, err
}
