package notes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Handler struct{ repo *Repo }

func NewHandler(r *Repo) *Handler { return &Handler{repo: r} }

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/stats", h.stats)
	r.Get("/{id}", h.get)
	r.Patch("/{id}", h.patch)
	r.Delete("/{id}", h.del)
	return r
}

func reqCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 5*time.Second)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}

func repoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	case mongo.IsDuplicateKeyError(err):
		writeError(w, http.StatusConflict, "title_already_exists")
	default:
		writeError(w, http.StatusInternalServerError, "database_error")
	}
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title     string     `json:"title"`
		Content   string     `json:"content"`
		ExpiresAt *time.Time `json:"expiresAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Title) == "" {
		writeError(w, http.StatusBadRequest, "invalid_json_or_title")
		return
	}
	ctx, cancel := reqCtx(r)
	defer cancel()
	n, err := h.repo.Create(ctx, strings.TrimSpace(in.Title), in.Content, in.ExpiresAt)
	if err != nil {
		repoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	n, err := h.repo.ByID(ctx, chi.URLParam(r, "id"))
	if err != nil {
		repoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, err := strconv.ParseInt(query.Get("limit"), 10, 64)
	if query.Get("limit") != "" && (err != nil || limit <= 0 || limit > 200) {
		writeError(w, http.StatusBadRequest, "limit_must_be_1_to_200")
		return
	}
	if query.Get("limit") == "" {
		limit = 20
	}
	skip := int64(0)
	if query.Get("skip") != "" {
		skip, err = strconv.ParseInt(query.Get("skip"), 10, 64)
		if err != nil || skip < 0 {
			writeError(w, http.StatusBadRequest, "invalid_skip")
			return
		}
	}
	after := query.Get("after")
	if after != "" {
		if _, err := primitive.ObjectIDFromHex(after); err != nil || skip != 0 {
			writeError(w, http.StatusBadRequest, "invalid_after_or_skip")
			return
		}
	}
	ctx, cancel := reqCtx(r)
	defer cancel()
	items, err := h.repo.List(ctx, query.Get("q"), limit, skip, after)
	if err != nil {
		repoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title     *string    `json:"title"`
		Content   *string    `json:"content"`
		ExpiresAt *time.Time `json:"expiresAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" {
			writeError(w, http.StatusBadRequest, "title_required")
			return
		}
		in.Title = &title
	}
	ctx, cancel := reqCtx(r)
	defer cancel()
	n, err := h.repo.Update(ctx, chi.URLParam(r, "id"), in.Title, in.Content, in.ExpiresAt)
	if err != nil {
		repoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (h *Handler) del(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	if err := h.repo.Delete(ctx, chi.URLParam(r, "id")); err != nil {
		repoError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	result, err := h.repo.Statistics(ctx)
	if err != nil {
		repoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
