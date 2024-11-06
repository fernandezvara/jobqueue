package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/fernandezvara/jobqueues/internal/queue"
	"github.com/fernandezvara/jobqueues/internal/storage"
	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	service queue.Service
}

func NewHandlers(service queue.Service) *Handlers {
	return &Handlers{service: service}
}

func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := struct {
		Status    string    `json:"status"`
		Version   string    `json:"version"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Status:    "ok",
		Version:   "0.2.0",
		Timestamp: time.Now(),
	}

	respondJSON(w, http.StatusOK, status)
}

func (h *Handlers) GetQueue(w http.ResponseWriter, r *http.Request) {
	queueName := chi.URLParam(r, "name")
	queue, err := h.service.GetQueue(r.Context(), queueName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if queue == nil {
		respondError(w, http.StatusNotFound, "queue not found")
		return
	}

	respondJSON(w, http.StatusOK, queue)
}

func (h *Handlers) GetQueues(w http.ResponseWriter, r *http.Request) {
	var (
		queues []storage.Queue
		err    error
	)
	queues, err = h.service.GetQueues(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if queues == nil {
		queues = []storage.Queue{}
	}

	respondJSON(w, http.StatusOK, queues)
}

func (h *Handlers) CreateOrUpdateQueue(w http.ResponseWriter, r *http.Request) {
	var queue storage.Queue
	if err := json.NewDecoder(r.Body).Decode(&queue); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	queue.Name = chi.URLParam(r, "name")

	// validation
	if queue.TaskTimeout <= 0 {
		respondError(w, http.StatusBadRequest, "Task timeout must be positive")
		return
	}

	if err := h.service.CreateOrUpdateQueue(r.Context(), &queue); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, queue)
}

func (h *Handlers) CreateTask(w http.ResponseWriter, r *http.Request) {
	var task storage.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.service.CreateTask(r.Context(), &task); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, task)
}

func (h *Handlers) GetTasks(w http.ResponseWriter, r *http.Request) {
	filter := storage.TaskFilter{
		QueueName: r.URL.Query().Get("queue"),
		Status:    r.URL.Query().Get("status"),
		SortBy:    r.URL.Query().Get("sort_by"),
	}

	if from := r.URL.Query().Get("from"); from != "" {
		fromTime, err := strconv.ParseInt(from, 10, 64)
		if err == nil {
			filter.FromDate = time.Unix(fromTime, 0)
		}
	}

	if to := r.URL.Query().Get("to"); to != "" {
		toTime, err := strconv.ParseInt(to, 10, 64)
		if err == nil {
			filter.ToDate = time.Unix(toTime, 0)
		}
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		filter.Limit, _ = strconv.Atoi(limit)
	}

	if offset := r.URL.Query().Get("offset"); offset != "" {
		filter.Offset, _ = strconv.Atoi(offset)
	}

	// Comprobar si se solicita el resumen
	summary := r.URL.Query().Get("summary") == "true"

	if summary {
		stats, err := h.service.GetTaskStats(r.Context(), filter)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, stats)
		return
	}

	// Comportamiento normal - devolver lista de tareas
	tasks, err := h.service.GetTasks(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if tasks == nil {
		tasks = []storage.Task{}
	}

	respondJSON(w, http.StatusOK, tasks)
}

func (h *Handlers) UpdateTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")

	var task storage.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	task.ID = taskID
	if err := h.service.UpdateTask(r.Context(), &task); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, task)
}

func (h *Handlers) DeleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")

	if err := h.service.DeleteTask(r.Context(), taskID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) GetNextTask(w http.ResponseWriter, r *http.Request) {
	queueName := r.URL.Query().Get("queue")
	clientID := r.Header.Get("X-Client-ID")

	task, err := h.service.GetNextTask(r.Context(), queueName, clientID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if task == nil {
		respondError(w, http.StatusNotFound, "No tasks available")
		return
	}

	respondJSON(w, http.StatusOK, task)
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, code int, message string) {
	respondJSON(w, code, map[string]string{"error": message})
}
