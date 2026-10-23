package employee_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/go-chi/chi/v5"
	"github.com/vasmaae/distributed-computing-and-applications/common/auth"
	errs "github.com/vasmaae/distributed-computing-and-applications/employees/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/handler"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/handler/dto"
	employeehandler "github.com/vasmaae/distributed-computing-and-applications/employees/internal/handler/employee"
	reporthandler "github.com/vasmaae/distributed-computing-and-applications/employees/internal/handler/report"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/model"
)

type sortService struct {
	employeehandler.Service
	field, order string
	threads      int
	employees    []model.Employee
	err          error
	calls        int
}

func (s *sortService) GetSorted(_ context.Context, field, order string, threads int) ([]model.Employee, time.Duration, error) {
	s.field, s.order, s.threads = field, order, threads
	s.calls++
	return s.employees, 125 * time.Microsecond, s.err
}

func TestSortHandlerResponse(t *testing.T) {
	fio, err := model.NewFIO("Иванов Иван")
	if err != nil {
		t.Fatal(err)
	}
	for _, employees := range [][]model.Employee{nil, {{ID: uuid.NewV7(), FIO: fio, BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)}}} {
		svc := &sortService{employees: employees}
		router := chi.NewRouter()
		handler.RegisterRoutes(router, employeehandler.NewHandler(svc), reporthandler.NewHandler(nil, nil))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/employees/sorted", strings.NewReader(`{"sort_by":"fio","order":"desc","threads":5}`)))
		if w.Code != http.StatusOK || svc.field != "fio" || svc.order != "desc" || svc.threads != 5 || svc.calls != 1 {
			t.Fatalf("status=%d service=%+v body=%s", w.Code, svc, w.Body)
		}
		var got dto.SortedEmployeesResponse
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Employees == nil || len(got.Employees) != len(employees) || got.SortDurationMS != 0.125 {
			t.Fatalf("response=%+v", got)
		}
		if len(employees) > 0 && got.Employees[0].ID != employees[0].ID.String() {
			t.Fatal("employee data was lost")
		}
	}
}

func TestSortHandlerInvalidBody(t *testing.T) {
	for _, body := range []string{
		"", "{", "[]", `{"threads":"5"}`, `{"threads":1.5}`,
		`{"sort_by":"id","order":"asc","threads":5,"extra":true}`,
		`{"sort_by":"id","order":"asc","threads":5} {}`,
		`{"sort_by":"` + strings.Repeat("x", 4096) + `","order":"asc","threads":5}`,
	} {
		svc := &sortService{}
		w := httptest.NewRecorder()
		employeehandler.NewHandler(svc).GetSorted(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest || svc.calls != 0 {
			t.Fatalf("body=%q: status=%d calls=%d", body, w.Code, svc.calls)
		}
	}
}

func TestSortHandlerErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{errs.ErrInvalidSort, http.StatusBadRequest},
		{errors.New("secret database details"), http.StatusInternalServerError},
	} {
		w := httptest.NewRecorder()
		employeehandler.NewHandler(&sortService{err: tc.err}).GetSorted(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"sort_by":"id","order":"asc","threads":5}`)))
		if w.Code != tc.status || strings.Contains(w.Body.String(), "secret database details") {
			t.Fatalf("status=%d body=%s", w.Code, w.Body)
		}
	}
}

func TestSortRouteRequiresJWT(t *testing.T) {
	authenticator, err := auth.New(strings.Repeat("s", 32))
	if err != nil {
		t.Fatal(err)
	}
	svc := &sortService{}
	router := chi.NewRouter()
	router.Use(authenticator.Middleware)
	handler.RegisterRoutes(router, employeehandler.NewHandler(svc), reporthandler.NewHandler(nil, nil))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/employees/sorted", strings.NewReader(`{"sort_by":"id","order":"asc","threads":5}`)))
	if w.Code != http.StatusUnauthorized || svc.calls != 0 {
		t.Fatalf("status=%d calls=%d", w.Code, svc.calls)
	}
}
