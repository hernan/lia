package admin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"urlshortener/internal/session"
	"urlshortener/store"
)

type mockStore struct {
	urls      []*store.URL
	listErr   error
	createErr error
	updateErr error
	deleteErr error
	getErr    error
	nextID    int64
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
	if m.createErr != nil {
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
	defer srv.Close()

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
	a, sm := newTestAdmin(t, s)

	mux := http.NewServeMux()
	a.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

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
	_ = sm

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
	defer srv.Close()

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
	defer srv.Close()

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
	defer srv.Close()

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
	defer srv.Close()

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
	defer srv.Close()

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
	defer srv.Close()

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
	defer srv.Close()

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
	defer srv.Close()

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
	defer srv.Close()

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
	defer srv.Close()

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
	_ = sm
}
