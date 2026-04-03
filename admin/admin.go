package admin

import (
	"bytes"
	"context"
	"crypto/subtle"
	"embed"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"urlshortener/internal/session"
	"urlshortener/shortener"
	"urlshortener/store"
)

type ctxKey struct{}

type sessionCtx struct {
	Flash      string
	FlashError bool
}

//go:embed templates/*.html
var templateFS embed.FS

// URLStore is the interface the admin package requires from the store.
type URLStore interface {
	List() ([]*store.URL, error)
	Search(query string) ([]*store.URL, error)
	GetByID(id int64) (*store.URL, error)
	Create(originalURL, code string) (*store.URL, error)
	Update(id int64, originalURL string) error
	Delete(id int64) error
}

// Admin handles the admin panel HTTP requests.
type Admin struct {
	store    URLStore
	sessions *session.Manager
	tmpls    *template.Template
	username string
	password string
}

// New creates an Admin instance.
func New(s URLStore, sm *session.Manager, username, password string) (*Admin, error) {
	tmpls, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Admin{
		store:    s,
		sessions: sm,
		tmpls:    tmpls,
		username: username,
		password: password,
	}, nil
}

func validURL(s string) bool {
	parsed, err := url.Parse(s)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	return true
}

// RegisterRoutes registers admin routes on the given mux.
// Routes are prefixed with /admin.
func (a *Admin) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/login", a.Login)
	mux.HandleFunc("POST /admin/login", a.Login)
	mux.Handle("POST /admin/logout", a.requireAuth(a.requireCSRF(http.HandlerFunc(a.Logout))))
	mux.Handle("GET /admin", a.requireAuth(http.HandlerFunc(a.Dashboard)))
	mux.Handle("POST /admin", a.requireAuth(a.requireCSRF(http.HandlerFunc(a.CreateURL))))
	mux.Handle("GET /admin/urls/{id}/edit", a.requireAuth(http.HandlerFunc(a.EditURL)))
	mux.Handle("POST /admin/urls/{id}/edit", a.requireAuth(a.requireCSRF(http.HandlerFunc(a.UpdateURL))))
	mux.Handle("POST /admin/urls/{id}/delete", a.requireAuth(a.requireCSRF(http.HandlerFunc(a.DeleteURL))))
}

func (a *Admin) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, flash, flashError, err := a.sessions.Validate(r)
		if err != nil {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, sessionCtx{
			Flash:      flash,
			FlashError: flashError,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Admin) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !session.VerifyCSRF(r) {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *Admin) setCSRF(w http.ResponseWriter) (string, error) {
	token, err := session.GenerateToken()
	if err != nil {
		return "", err
	}
	session.SetCSRFCookie(w, token)
	return token, nil
}

// render executes a named template into a buffer and, on success, writes it to
// w with Content-Type text/html. If execution fails the error is logged and a
// 500 response is sent instead, avoiding a partial/committed response.
func (a *Admin) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := a.tmpls.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("admin: render template %q: %v", name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

type loginData struct {
	Error     string
	CSRFToken string
}

func (a *Admin) Login(w http.ResponseWriter, r *http.Request) {
	csrfToken, err := a.setCSRF(w)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		a.render(w, "login.html", loginData{CSRFToken: csrfToken})
		return
	}

	if !session.VerifyCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	usernameOK := subtle.ConstantTimeCompare([]byte(username), []byte(a.username)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) == 1

	if !usernameOK || !passwordOK {
		a.render(w, "login.html", loginData{
			Error:     "Invalid credentials",
			CSRFToken: csrfToken,
		})
		return
	}

	http.SetCookie(w, a.sessions.Create(username))
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (a *Admin) Logout(w http.ResponseWriter, r *http.Request) {
	a.sessions.Destroy(w)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

type dashboardData struct {
	URLs       []*store.URL
	Query      string
	Flash      string
	FlashError bool
	CSRFToken  string
}

func (a *Admin) Dashboard(w http.ResponseWriter, r *http.Request) {
	csrfToken, err := a.setCSRF(w)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	query := r.URL.Query().Get("q")
	s, ok := r.Context().Value(ctxKey{}).(sessionCtx)
	if !ok {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var urls []*store.URL
	if query != "" {
		urls, err = a.store.Search(query)
	} else {
		urls, err = a.store.List()
	}
	if err != nil {
		log.Printf("admin: list urls: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	a.sessions.ClearFlash(w, r)

	a.render(w, "dashboard.html", dashboardData{
		URLs:       urls,
		Query:      query,
		Flash:      s.Flash,
		FlashError: s.FlashError,
		CSRFToken:  csrfToken,
	})
}

func (a *Admin) CreateURL(w http.ResponseWriter, r *http.Request) {
	originalURL := strings.TrimSpace(r.FormValue("url"))
	if originalURL == "" {
		a.sessions.SetFlash(w, r, "url is required", true)
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	if !validURL(originalURL) {
		a.sessions.SetFlash(w, r, "invalid url", true)
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	var err error
	for range 5 {
		code := shortener.Generate()
		_, err = a.store.Create(originalURL, code)
		if err == nil {
			break
		}
		if !errors.Is(err, store.ErrConflict) {
			log.Printf("admin: create url: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	if err != nil {
		log.Printf("admin: create url: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	a.sessions.SetFlash(w, r, "URL created", false)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

type editData struct {
	URL       *store.URL
	Error     string
	CSRFToken string
}

func (a *Admin) EditURL(w http.ResponseWriter, r *http.Request) {
	csrfToken, err := a.setCSRF(w)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	u, err := a.store.GetByID(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	a.render(w, "edit.html", editData{
		URL:       u,
		CSRFToken: csrfToken,
	})
}

func (a *Admin) UpdateURL(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	originalURL := strings.TrimSpace(r.FormValue("url"))
	if originalURL == "" {
		a.renderEditError(w, r, id, "url is required")
		return
	}
	if !validURL(originalURL) {
		a.renderEditError(w, r, id, "invalid url")
		return
	}

	if err := a.store.Update(id, originalURL); err != nil {
		log.Printf("admin: update url: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	a.sessions.SetFlash(w, r, "URL updated", false)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (a *Admin) renderEditError(w http.ResponseWriter, r *http.Request, id int64, msg string) {
	csrfToken, err := a.setCSRF(w)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	u, err := a.store.GetByID(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	a.render(w, "edit.html", editData{
		URL:       u,
		Error:     msg,
		CSRFToken: csrfToken,
	})
}

func (a *Admin) DeleteURL(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := a.store.Delete(id); err != nil {
		log.Printf("admin: delete url: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	a.sessions.SetFlash(w, r, "URL deleted", false)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}
