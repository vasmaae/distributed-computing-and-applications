package dto

type ReportResponse struct {
	Employees []EmployeeResponse `json:"employees"`
	Total     int                `json:"total"`
	IsFired   int                `json:"is_fired"`
}

type EmployeeResponse struct {
	ID        string `json:"id"`
	FIO       string `json:"fio"`
	BirthDate string `json:"birth_date"`
	IsFired   bool   `json:"is_fired"`
}
