package report

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/go-chi/chi/v5"

	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/client/file"
)

func TestGetFileStatusAndContent(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNotFound, http.StatusBadGateway} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			if status == http.StatusOK {
				_, _ = w.Write([]byte("id,fio\n"))
			}
		}))
		router := chi.NewRouter()
		router.Get("/files/{id}", NewHandler(nil, file.NewClient(server.URL)).GetFile)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/files/"+uuid.New().String(), nil))
		server.Close()
		if w.Code != status {
			t.Fatalf("status=%d want=%d", w.Code, status)
		}
		if status == http.StatusOK && (w.Body.String() != "id,fio\n" || w.Header().Get("Content-Type") != "text/csv; charset=utf-8") {
			t.Fatal("CSV response is invalid")
		}
		w = httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/files/wrong", nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid UUID: status=%d", w.Code)
		}
	}
}
