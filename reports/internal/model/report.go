package model

type Report struct {
	Employees []Employee
	Total     int
	IsFired   int
}

func NewReport(employees []Employee) Report {
	return Report{
		Employees: employees,
		Total:     len(employees),
		IsFired:   countFiredEmployees(employees),
	}
}

func countFiredEmployees(employees []Employee) (count int) {
	for _, e := range employees {
		if e.IsFired {
			count++
		}
	}

	return count
}
