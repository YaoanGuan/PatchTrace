package marketplace

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
)

const maxBodyBytes = 1 << 20

// Handler binds the HTTP endpoints to a Store.
type Handler struct {
	store *Store
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) me(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.store.Me())
}

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	query := TaskQuery{Page: 1}

	if raw := values.Get("status"); raw != "" && raw != "ALL" {
		status := TaskStatus(raw)
		if !ValidTaskStatus(status) {
			writeError(w, http.StatusBadRequest, "invalid status")
			return
		}
		query.Status = status
	}
	query.Keyword = values.Get("keyword")

	if raw := values.Get("rewardMin"); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid rewardMin")
			return
		}
		query.RewardMin = &value
	}
	if raw := values.Get("rewardMax"); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid rewardMax")
			return
		}
		query.RewardMax = &value
	}
	if raw := values.Get("page"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			writeError(w, http.StatusBadRequest, "invalid page")
			return
		}
		query.Page = value
	}
	if raw := values.Get("pageSize"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			writeError(w, http.StatusBadRequest, "invalid pageSize")
			return
		}
		query.PageSize = value
	}

	writeJSON(w, http.StatusOK, h.store.ListTasks(query))
}

func (h *Handler) publishTask(w http.ResponseWriter, r *http.Request) {
	var input PublishTaskInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	detail, err := h.store.PublishTask(input)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

func (h *Handler) getTask(w http.ResponseWriter, r *http.Request) {
	detail, ok := h.store.GetTask(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) acceptTask(w http.ResponseWriter, r *http.Request) {
	response, err := h.store.AcceptTask(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) getTrace(w http.ResponseWriter, r *http.Request) {
	trace, ok := h.store.GetTrace(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, trace)
}

func (h *Handler) wallet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.store.Wallet())
}

func (h *Handler) reputation(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.Reputation(r.PathValue("userId")))
}

func (h *Handler) submitRating(w http.ResponseWriter, r *http.Request) {
	var input SubmitRatingInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rating, err := h.store.SubmitRating(input)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rating)
}

func (h *Handler) registerChannel(w http.ResponseWriter, r *http.Request) {
	channel := Channel(r.PathValue("channel"))

	var input RegisterChannelInput
	err := decodeJSON(r, &input)
	if err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	response, err := h.store.RegisterChannel(channel, input.Evidence)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) signContract(w http.ResponseWriter, r *http.Request) {
	var input SignContractInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	input.TaskID = r.PathValue("id")

	response, err := h.store.SignContract(input)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) notFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not found")
}

func decodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	return decoder.Decode(destination)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeStoreError(w http.ResponseWriter, err error) {
	var domainError *Error
	if errors.As(err, &domainError) {
		writeError(w, domainError.Status, domainError.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal server error")
}
