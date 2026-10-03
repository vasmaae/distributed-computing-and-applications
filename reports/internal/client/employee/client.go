package employee

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/client/dto"
	errs "github.com/vasmaae/distributed-computing-and-applications/reports/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/model"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/service/report"
)

var _ report.Repository = (*Client)(nil)

type Client struct {
	URL string
}

func NewClient(url string) *Client {
	return &Client{URL: url}
}

func (c *Client) GetAll(ctx context.Context) ([]model.Employee, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/api/v1/employees", c.URL),
		nil,
	)
	if err != nil {
		return nil, errs.ErrEmployeesClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errs.ErrEmployeesClient
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, errs.ErrEmployeesClient
	}

	var employeeResponses []dto.EmployeeResponse
	if err := json.NewDecoder(resp.Body).Decode(&employeeResponses); err != nil {
		return nil, errs.ErrEmployeesClient
	}

	employees, err := dto.ToEmployees(employeeResponses)
	return employees, err
}
