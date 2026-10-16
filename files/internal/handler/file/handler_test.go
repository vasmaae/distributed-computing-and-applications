package file

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/go-chi/chi/v5"

	errs "github.com/vasmaae/distributed-computing-and-applications/files/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/files/internal/handler"
)

type memoryStorage map[uuid.UUID]string

func (s memoryStorage) Save(_ context.Context, id uuid.UUID, src io.Reader) error {
	data, err := io.ReadAll(src)
	s[id] = string(data)
	return err
}

func (s memoryStorage) Get(_ context.Context, id uuid.UUID, dst io.Writer) error {
	data, ok := s[id]
	if !ok {
		return errs.ErrNotFound
	}
	_, err := io.WriteString(dst, data)
	return err
}

func TestFileRoutes(t *testing.T) {
	router := chi.NewRouter()
	handler.RegisterRoutes(router, NewHandler(memoryStorage{}))
	path := "/api/v1/files/" + uuid.New().String()
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodGet, path, "", http.StatusNotFound},
		{http.MethodPut, path, "", http.StatusBadRequest},
		{http.MethodPut, path, "id,fio\n1,test\n", http.StatusCreated},
		{http.MethodGet, path, "", http.StatusOK},
		{http.MethodGet, "/api/v1/files/wrong", "", http.StatusBadRequest},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if recorder.Code != tc.status {
			t.Fatalf("%s %s: status=%d body=%s", tc.method, tc.path, recorder.Code, recorder.Body)
		}
		if tc.method == http.MethodGet && tc.status == http.StatusOK && recorder.Body.String() != "id,fio\n1,test\n" {
			t.Fatal("download differs from upload")
		}
	}
}
