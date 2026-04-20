package api

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	_ "embed"

	"github.com/gappylul/flagd/internal/store"
)

//go:embed templates/page.html
var pageSrc string

//go:embed templates/flag_row.html
var flagRowSrc string

//go:embed templates/static/style.css
var styleCSS []byte

var (
	pageTmpl    = template.Must(template.New("page").Parse(pageSrc))
	flagRowTmpl = template.Must(template.New("row").Parse(flagRowSrc))
)

func (h *Handler) uiRoutes() {
	h.mux.HandleFunc("GET /ui", h.uiPage)
	h.mux.HandleFunc("GET /ui/flags", h.uiListFlags)
	h.mux.HandleFunc("POST /ui/flags", h.uiCreateFlag)
	h.mux.HandleFunc("PATCH /ui/flags/{name}/toggle", h.uiToggleFlag)
	h.mux.HandleFunc("DELETE /ui/flags/{name}", h.uiDeleteFlag)

	h.mux.HandleFunc("GET /static/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write(styleCSS)
	})
}

func (h *Handler) uiPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	pageTmpl.Execute(w, nil)
}

func (h *Handler) uiListFlags(w http.ResponseWriter, r *http.Request) {
	flags, err := h.store.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(flags) == 0 {
		fmt.Fprint(w, `<p class="empty">no flags yet — create one above</p>`)
		return
	}
	var b strings.Builder
	for _, f := range flags {
		flagRowTmpl.Execute(&b, f)
	}
	fmt.Fprint(w, b.String())
}

func (h *Handler) uiCreateFlag(w http.ResponseWriter, r *http.Request) {
	if !h.checkKey(w, r) {
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	enabled := r.FormValue("enabled") == "true"

	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	flag, err := h.store.Upsert(r.Context(), name, description, enabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	flagRowTmpl.Execute(w, flag)
}

func (h *Handler) uiToggleFlag(w http.ResponseWriter, r *http.Request) {
	if !h.checkKey(w, r) {
		return
	}
	flag, err := h.store.Toggle(r.Context(), r.PathValue("name"))
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	flagRowTmpl.Execute(w, flag)
}

func (h *Handler) uiDeleteFlag(w http.ResponseWriter, r *http.Request) {
	if !h.checkKey(w, r) {
		return
	}
	err := h.store.Delete(r.Context(), r.PathValue("name"))
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) checkKey(w http.ResponseWriter, r *http.Request) bool {
	if h.adminKey == "" {
		return true
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token != h.adminKey {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}
