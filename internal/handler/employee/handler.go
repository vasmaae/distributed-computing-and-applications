package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
	"uuid"

	errs "github.com/vasmaae/distributed-computing-and-applications/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/internal/handler/dto"
	"github.com/vasmaae/distributed-computing-and-applications/internal/handler/request"
	"github.com/vasmaae/distributed-computing-and-applications/internal/handler/response"
	"github.com/vasmaae/distributed-computing-and-applications/internal/model"
)

type Service interface {
	GetByID(ctx context.Context, id uuid.UUID) (model.Employee, error)
	GetAll(ctx context.Context) ([]model.Employee, error)
	Create(ctx context.Context, fio string, birthDate time.Time) error
	Update(ctx context.Context, id uuid.UUID, fio string, birthDate time.Time) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type Handler struct {
	s Service
}

func NewHandler(s Service) *Handler {
	return &Handler{s: s}
}

// GetByID
// @Summary Get employee by ID
// @Tags employees
// @Produce JSON
// @Param id path string true "Employee ID"
// @Success 200 {object} dto.EmployeeResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /employees/{id} [get]
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(request.ReadValue(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "id is missing or invalid")
		return
	}

	employee, err := h.s.GetByID(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrEmployeeNotFound):
			response.WriteError(w, http.StatusNotFound, err.Error())
		default:
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	employeeResponse := dto.ToEmployeeResponse(employee)
	response.WriteResponse(w, http.StatusOK, employeeResponse)
}

// GetAll
// @Summary Get all employees
// @Tags employees
// @Produce JSON
// @Success 200 {array} dto.EmployeeResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /employees [get]
func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	employees, err := h.s.GetAll(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	employeeResponses := dto.ToEmployeeResponses(employees)
	response.WriteResponse(w, http.StatusOK, employeeResponses)
}

// Create
// @Summary Create employee
// @Tags employees
// @Accept JSON
// @Produce JSON
// @Param employee body dto.CreateEmployeeRequest true "Employee data"
// @Success 201 {object} dto.CreateEmployeeRequest
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /employees [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var employee dto.CreateEmployeeRequest

	if err := json.NewDecoder(r.Body).Decode(&employee); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	birthDate, err := time.Parse(time.RFC3339, employee.BirthDate)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "wrong date format")
		return
	}

	err = h.s.Create(r.Context(), employee.FIO, birthDate)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrInvalidBirthDate) ||
			errors.Is(err, errs.ErrInvalidFIO):
			response.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.WriteResponse(w, http.StatusCreated, employee)
}

// Update
// @Summary Update employee
// @Tags employees
// @Accept JSON
// @Produce JSON
// @Param id path string true "Employee ID"
// @Param employee body dto.UpdateEmployeeRequest true "Employee data"
// @Success 200 {object} dto.UpdateEmployeeRequest
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /employees/{id} [put]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var employee dto.UpdateEmployeeRequest

	if err := json.NewDecoder(r.Body).Decode(&employee); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	id, err := uuid.Parse(request.ReadValue(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "id is invalid")
		return
	}

	birthDate, err := time.Parse(time.RFC3339, employee.BirthDate)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "wrong date format")
		return
	}

	err = h.s.Update(r.Context(), id, employee.FIO, birthDate)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrEmployeeNotFound):
			response.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, errs.ErrInvalidBirthDate) ||
			errors.Is(err, errs.ErrInvalidFIO):
			response.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.WriteResponse(w, http.StatusOK, employee)
}

// Delete
// @Summary Delete employee
// @Tags employees
// @Param id path string true "Employee ID"
// @Success 204
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /employees/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(request.ReadValue(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "id is missing or invalid")
		return
	}

	err = h.s.Delete(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrEmployeeNotFound):
			response.WriteError(w, http.StatusNotFound, err.Error())
		default:
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.WriteHeader(w, http.StatusNoContent)
}
