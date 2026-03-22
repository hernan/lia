package admin

import (
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"urlshortener/internal/session"
	"urlshortener/store"
)

type mockStore struct {
	urls           []*store.URL
	listErr        error
	createErr      error
	createErrTimes int // when > 0, return createErr this many times then succeed
	updateErr      error
	deleteErr      error
	getErr         error
	nextID         int64
}

func (m *mockStore) List() ([]*store.URL, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.urls, nil
}

func (m *mockStore) Search(query string) ([]*store.URL, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*store.URL
	for _, u := range m.urls {
		if strings.Contains(u.OriginalURL, query) {
			result = append(result, u)
		}
	}
	return result, nil
}

func (m *mockStore) GetByID(id int64) (*store.URL, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, u := range m.urls {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockStore) Create(originalURL, code string) (*store.URL, error) {
	if m.createErr != nil && m.createErrTimes > 0 {
		m.createErrTimes--
		return nil, m.createErr
	}
	m.nextID++
	u := &store.URL{ID: m.nextID, Code: code, OriginalURL: originalURL}
	m.urls = append(m.urls, u)
	return u, nil
}

func (m *mockStore) Update(id int64, originalURL string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for _, u := range m.urls {
		if u.ID == id {
			u.OriginalURL = originalURL
			return nil
		}
	}
	return errors.New("not found")
}

func (m *mockStore) Delete(id int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for i, u := range m.urls {
		if u.ID == id {
			m.urls = append(m.urls[:i], m.urls[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

func newTestAdmin(t *testing.T, s *mockStore) (*Admin, *session.Manager) {
	t.Helper()
	sm := session.New([]byte("test-secret"))
	a, err := New(s, sm, "admin", "password123")
	if err != nil {
		t.Fatal(err)
	}
	return a, sm
}

func loginSession(t *testing.T, sm *session.Manager) *http.Cookie {
	t.Helper()
	return sm.Create("admin")
}

func csrfCookieAndForm() (*http.Cookie, url.Values) {
	token, _ := session.GenerateToken()
	cookie := &http.Cookie{Name: "csrf_token", Value: token}
	form := url.Values{}
	form.Set("csrf_token", token)
	return cookie, form
}

func TestLoginGet(t *testing.T) {
	s := &mockStore{}
	a, _ := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/admin/login")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLoginPostInvalidCredentials(t *testing.T) {
	s := &mockStore{}
	a, _ := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// Get CSRF token first
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	// Post with wrong credentials
	csrfToken, _ := session.GenerateToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)
	form.Set("username", "wrong")
	form.Set("password", "wrong")

	req, _ := http.NewRequest("POST", srv.URL+"/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 (re-render login), got %d", resp.StatusCode)
	}
}

func TestLoginPostValidCredentials(t *testing.T) {
	s := &mockStore{}
	a, _ := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	csrfToken, _ := session.GenerateToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)
	form.Set("username", "admin")
	form.Set("password", "password123")

	req, _ := http.NewRequest("POST", srv.URL+"/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}

	// Should have session cookie
	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			found = true
		}
	}
	if !found {
		t.Error("expected session cookie in response")
	}
}

func TestDashboardRequiresAuth(t *testing.T) {
	s := &mockStore{}
	a, _ := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	resp, err := client.Get(srv.URL + "/admin")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", resp.StatusCode)
	}
}

func TestDashboardWithAuth(t *testing.T) {
	s := &mockStore{
		urls: []*store.URL{
			{ID: 1, Code: "abc123", OriginalURL: "https://example.com"},
		},
	}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	req, _ := http.NewRequest("GET", srv.URL+"/admin", nil)
	req.AddCookie(loginSession(t, sm))

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestDashboardSearch(t *testing.T) {
	s := &mockStore{
		urls: []*store.URL{
			{ID: 1, Code: "abc", OriginalURL: "https://golang.org"},
			{ID: 2, Code: "def", OriginalURL: "https://example.com"},
		},
	}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	req, _ := http.NewRequest("GET", srv.URL+"/admin?q=golang", nil)
	req.AddCookie(loginSession(t, sm))

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCreateURL(t *testing.T) {
	s := &mockStore{}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	csrfCookie, form := csrfCookieAndForm()
	form.Set("url", "https://newsite.com")

	req, _ := http.NewRequest("POST", srv.URL+"/admin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(loginSession(t, sm))
	req.AddCookie(csrfCookie)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}
	if len(s.urls) != 1 {
		t.Errorf("expected 1 URL in store, got %d", len(s.urls))
	}
}

func TestCreateURLRequiresCSRF(t *testing.T) {
	s := &mockStore{}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	form := url.Values{}
	form.Set("url", "https://newsite.com")

	req, _ := http.NewRequest("POST", srv.URL+"/admin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(loginSession(t, sm))

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCreateURLInvalidURL(t *testing.T) {
	s := &mockStore{}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	csrfCookie, form := csrfCookieAndForm()
	form.Set("url", "not a url")

	req, _ := http.NewRequest("POST", srv.URL+"/admin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(loginSession(t, sm))
	req.AddCookie(csrfCookie)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}
	if len(s.urls) != 0 {
		t.Errorf("expected 0 URLs in store, got %d", len(s.urls))
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "flash_error=1") {
		t.Errorf("expected redirect with flash_error, got %s", loc)
	}
}

func TestCreateURLMissingScheme(t *testing.T) {
	s := &mockStore{}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	csrfCookie, form := csrfCookieAndForm()
	form.Set("url", "example.com")

	req, _ := http.NewRequest("POST", srv.URL+"/admin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(loginSession(t, sm))
	req.AddCookie(csrfCookie)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}
	if len(s.urls) != 0 {
		t.Errorf("expected 0 URLs in store, got %d", len(s.urls))
	}
}

func TestCreateURLDangerousScheme(t *testing.T) {
	s := &mockStore{}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	csrfCookie, form := csrfCookieAndForm()
	form.Set("url", "javascript:alert(1)")

	req, _ := http.NewRequest("POST", srv.URL+"/admin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(loginSession(t, sm))
	req.AddCookie(csrfCookie)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}
	if len(s.urls) != 0 {
		t.Errorf("expected 0 URLs in store, got %d", len(s.urls))
	}
}

func TestEditURL(t *testing.T) {
	s := &mockStore{
		urls: []*store.URL{
			{ID: 1, Code: "abc123", OriginalURL: "https://old.com"},
		},
	}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	req, _ := http.NewRequest("GET", srv.URL+"/admin/urls/1/edit", nil)
	req.AddCookie(loginSession(t, sm))

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUpdateURL(t *testing.T) {
	s := &mockStore{
		urls: []*store.URL{
			{ID: 1, Code: "abc123", OriginalURL: "https://old.com"},
		},
	}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	csrfCookie, form := csrfCookieAndForm()
	form.Set("url", "https://new.com")

	req, _ := http.NewRequest("POST", srv.URL+"/admin/urls/1/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(loginSession(t, sm))
	req.AddCookie(csrfCookie)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}
	if s.urls[0].OriginalURL != "https://new.com" {
		t.Errorf("expected URL updated to https://new.com, got %s", s.urls[0].OriginalURL)
	}
}

func TestUpdateURLInvalidURL(t *testing.T) {
	s := &mockStore{
		urls: []*store.URL{
			{ID: 1, Code: "abc123", OriginalURL: "https://old.com"},
		},
	}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	csrfCookie, form := csrfCookieAndForm()
	form.Set("url", "not a url")

	req, _ := http.NewRequest("POST", srv.URL+"/admin/urls/1/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(loginSession(t, sm))
	req.AddCookie(csrfCookie)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if s.urls[0].OriginalURL != "https://old.com" {
		t.Errorf("expected URL unchanged, got %s", s.urls[0].OriginalURL)
	}
}

func TestUpdateURLMissingScheme(t *testing.T) {
	s := &mockStore{
		urls: []*store.URL{
			{ID: 1, Code: "abc123", OriginalURL: "https://old.com"},
		},
	}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	csrfCookie, form := csrfCookieAndForm()
	form.Set("url", "example.com")

	req, _ := http.NewRequest("POST", srv.URL+"/admin/urls/1/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(loginSession(t, sm))
	req.AddCookie(csrfCookie)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if s.urls[0].OriginalURL != "https://old.com" {
		t.Errorf("expected URL unchanged, got %s", s.urls[0].OriginalURL)
	}
}

func TestDeleteURL(t *testing.T) {
	s := &mockStore{
		urls: []*store.URL{
			{ID: 1, Code: "abc123", OriginalURL: "https://example.com"},
		},
	}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	csrfCookie, form := csrfCookieAndForm()

	req, _ := http.NewRequest("POST", srv.URL+"/admin/urls/1/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(loginSession(t, sm))
	req.AddCookie(csrfCookie)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}
	if len(s.urls) != 0 {
		t.Errorf("expected 0 URLs after delete, got %d", len(s.urls))
	}
}

func TestLogout(t *testing.T) {
	s := &mockStore{}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	csrfCookie, form := csrfCookieAndForm()

	req, _ := http.NewRequest("POST", srv.URL+"/admin/logout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(loginSession(t, sm))
	req.AddCookie(csrfCookie)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}

	// Session cookie should be cleared
	var sessionCleared bool
	for _, c := range resp.Cookies() {
		if c.Name == "session" && c.MaxAge == -1 {
			sessionCleared = true
		}
	}
	if !sessionCleared {
		t.Error("expected session cookie to be cleared")
	}
}

func TestCreateURLConflictRetries(t *testing.T) {
	// createErrTimes = 2: the first two Create calls return ErrConflict;
	// the third succeeds. The handler retries up to 5 times, so it must
	// succeed and redirect 303.
	s := &mockStore{
		createErr:      store.ErrConflict,
		createErrTimes: 2,
	}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	csrfCookie, form := csrfCookieAndForm()
	form.Set("url", "https://example.com")

	req, _ := http.NewRequest("POST", srv.URL+"/admin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(loginSession(t, sm))
	req.AddCookie(csrfCookie)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303 after retrying conflicts, got %d", resp.StatusCode)
	}
	if len(s.urls) != 1 {
		t.Errorf("expected 1 URL created after retries, got %d", len(s.urls))
	}
}

func TestCreateURLConflictExhausted(t *testing.T) {
	// createErrTimes = 5: all five retry attempts return ErrConflict.
	// The handler must give up and return 500.
	s := &mockStore{
		createErr:      store.ErrConflict,
		createErrTimes: 5,
	}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	csrfCookie, form := csrfCookieAndForm()
	form.Set("url", "https://example.com")

	req, _ := http.NewRequest("POST", srv.URL+"/admin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(loginSession(t, sm))
	req.AddCookie(csrfCookie)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 when all retries exhausted, got %d", resp.StatusCode)
	}
	if len(s.urls) != 0 {
		t.Errorf("expected 0 URLs after exhausted retries, got %d", len(s.urls))
	}
}

func TestRenderTemplateError(t *testing.T) {
	s := &mockStore{
		urls: []*store.URL{
			{ID: 1, Code: "abc123", OriginalURL: "https://example.com"},
		},
	}
	a, sm := newTestAdmin(t, s)

	// Build a replacement template set where every named template always
	// returns an error during execution. The fail func is registered on the
	// root and inherited by every associated template.
	failFn := template.FuncMap{
		"fail": func() (string, error) { return "", errors.New("injected template error") },
	}
	broken := template.Must(
		template.New("dashboard.html").Funcs(failFn).Parse(`{{fail}}`),
	)
	template.Must(broken.New("login.html").Parse(`{{fail}}`))
	template.Must(broken.New("edit.html").Parse(`{{fail}}`))
	a.tmpls = broken

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	t.Run("dashboard returns 500", func(t *testing.T) {
		req, _ := http.NewRequest("GET", srv.URL+"/admin", nil)
		req.AddCookie(loginSession(t, sm))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("expected 500 on template error, got %d", resp.StatusCode)
		}
	})

	t.Run("login GET returns 500", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/admin/login")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("expected 500 on template error, got %d", resp.StatusCode)
		}
	})

	t.Run("edit returns 500", func(t *testing.T) {
		req, _ := http.NewRequest("GET", srv.URL+"/admin/urls/1/edit", nil)
		req.AddCookie(loginSession(t, sm))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("expected 500 on template error, got %d", resp.StatusCode)
		}
	})
}

func TestLogoutRequiresCSRF(t *testing.T) {
	s := &mockStore{}
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	// POST logout with a valid session but no CSRF token/cookie.
	req, _ := http.NewRequest("POST", srv.URL+"/admin/logout", nil)
	req.AddCookie(loginSession(t, sm))

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 without CSRF token, got %d", resp.StatusCode)
	}
}
