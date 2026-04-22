package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gappylul/flagd/internal/store"
)

type Handler struct {
	store    *store.Store
	mux      *http.ServeMux
	adminKey string
}

func New(s *store.Store, adminKey string) *Handler {
	h := &Handler{
		store:    s,
		mux:      http.NewServeMux(),
		adminKey: adminKey,
	}

	h.routes()
	h.uiRoutes()

	finalMux := http.NewServeMux()

	finalMux.Handle("/ui", h.mux)
	finalMux.Handle("/ui/", h.mux)
	finalMux.Handle("/static/", h.mux)

	finalMux.Handle("/", requireAuth(adminKey)(h.mux))

	h.mux = finalMux

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	h.mux.ServeHTTP(w, r)
}

func (h *Handler) routes() {
	h.mux.HandleFunc("GET /flags", h.listFlags)
	h.mux.HandleFunc("GET /flags/{name}", h.getFlag)
	h.mux.HandleFunc("PUT /flags/{name}", h.upsertFlag)
	h.mux.HandleFunc("PATCH /flags/{name}/toggle", h.toggleFlag)
	h.mux.HandleFunc("DELETE /flags/{name}", h.deleteFlag)
}

func (h *Handler) listFlags(w http.ResponseWriter, r *http.Request) {
	flags, err := h.store.List(r.Context())
	if err != nil {
		respond(w, http.StatusInternalServerError, errorBody(err))
		return
	}
	if flags == nil {
		flags = []store.Flag{}
	}
	respond(w, http.StatusOK, flags)
}

func (h *Handler) getFlag(w http.ResponseWriter, r *http.Request) {
	flag, err := h.store.Get(r.Context(), r.PathValue("name"))
	if errors.Is(err, store.ErrNotFound) {
		respond(w, http.StatusNotFound, errorBody(err))
		return
	}
	if err != nil {
		respond(w, http.StatusInternalServerError, errorBody(err))
		return
	}
	respond(w, http.StatusOK, flag)
}

func (h *Handler) upsertFlag(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body struct {
		Enabled     bool   `json:"enabled"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond(w, http.StatusBadRequest, errorBody(err))
		return
	}
	flag, err := h.store.Upsert(r.Context(), name, body.Description, body.Enabled)
	if err != nil {
		respond(w, http.StatusInternalServerError, errorBody(err))
		return
	}
	slog.Info("upserted flag", "name", name, "enabled", body.Enabled)
	respond(w, http.StatusOK, flag)
}

func (h *Handler) toggleFlag(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	flag, err := h.store.Toggle(r.Context(), name)
	if errors.Is(err, store.ErrNotFound) {
		respond(w, http.StatusNotFound, errorBody(err))
		return
	}
	if err != nil {
		respond(w, http.StatusInternalServerError, errorBody(err))
		return
	}
	slog.Info("toggled flag", "name", name, "enabled", flag.Enabled)
	respond(w, http.StatusOK, flag)
}

func (h *Handler) deleteFlag(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	err := h.store.Delete(r.Context(), name)
	if errors.Is(err, store.ErrNotFound) {
		respond(w, http.StatusNotFound, errorBody(err))
		return
	}
	if err != nil {
		respond(w, http.StatusInternalServerError, errorBody(err))
		return
	}
	slog.Info("deleted flag", "name", name)
	w.WriteHeader(http.StatusNoContent)
}

func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func errorBody(err error) map[string]string {
	return map[string]string{"error": err.Error()}
}
