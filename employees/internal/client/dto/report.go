package dto

type ReportResponse struct {
	FileID    string             `json:"file_id"`
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
