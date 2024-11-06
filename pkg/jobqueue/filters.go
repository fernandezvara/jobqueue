package jobqueue

import (
	"net/url"
	"strconv"
	"time"
)

// TaskFilter contains filters for searching tasks
type TaskFilter struct {
	QueueName string
	Status    string
	FromDate  time.Time
	ToDate    time.Time
	SortBy    string
	Offset    int
	Limit     int
}

// toQueryParams convierte el filtro en parámetros de consulta URL
func (f TaskFilter) toQueryParams() url.Values {
	params := url.Values{}

	// Agregar solo los parámetros que tienen valor
	if f.QueueName != "" {
		params.Set("queue", f.QueueName)
	}

	if f.Status != "" {
		params.Set("status", f.Status)
	}

	if !f.FromDate.IsZero() {
		params.Set("from", strconv.FormatInt(f.FromDate.Unix(), 10))
	}

	if !f.ToDate.IsZero() {
		params.Set("to", strconv.FormatInt(f.ToDate.Unix(), 10))
	}

	if f.SortBy != "" {
		params.Set("sort_by", f.SortBy)
	}

	if f.Offset > 0 {
		params.Set("offset", strconv.Itoa(f.Offset))
	}

	if f.Limit > 0 {
		params.Set("limit", strconv.Itoa(f.Limit))
	}

	return params
}

// NewTaskFilter crea un nuevo filtro con valores predeterminados
func NewTaskFilter() TaskFilter {
	return TaskFilter{
		Limit: 10, // valor predeterminado para el límite
	}
}

// WithQueue agrega un filtro por cola
func (f TaskFilter) WithQueue(queue string) TaskFilter {
	f.QueueName = queue
	return f
}

// WithStatus agrega un filtro por estado
func (f TaskFilter) WithStatus(status string) TaskFilter {
	f.Status = status
	return f
}

// WithDateRange agrega un filtro por rango de fechas
func (f TaskFilter) WithDateRange(from, to time.Time) TaskFilter {
	f.FromDate = from
	f.ToDate = to
	return f
}

// WithPagination agrega paginación al filtro
func (f TaskFilter) WithPagination(offset, limit int) TaskFilter {
	f.Offset = offset
	f.Limit = limit
	return f
}

// WithSort agrega ordenamiento al filtro
func (f TaskFilter) WithSort(sortBy string) TaskFilter {
	f.SortBy = sortBy
	return f
}
