package anna

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	colly "github.com/gocolly/colly/v2"
)

// Search needs an account session, not just a browser User-Agent.
var memberSessions = sessionCache{entries: make(map[[32]byte]cachedSession)}

type cachedSession struct {
	jar     http.CookieJar
	expires time.Time
}

type sessionCache struct {
	mu      sync.Mutex
	entries map[[32]byte]cachedSession
}

func (s *sessionCache) cookies(ctx context.Context, origin *url.URL, key string, force bool, transport http.RoundTripper) ([]*http.Cookie, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := sha256.Sum256([]byte(origin.String() + "\n" + key))
	if cached, ok := s.entries[id]; ok && !force && time.Now().Before(cached.expires) {
		if cookies := cached.jar.Cookies(origin); len(cookies) > 0 {
			return cookies, nil
		}
	}
	delete(s.entries, id)
	loginURL := origin.ResolveReference(&url.URL{Path: "/account/"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL.String(), strings.NewReader(url.Values{"key": {key}}.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create account sign-in request: %w", err)
	}
	req.Header.Set("User-Agent", BrowserUserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("account sign-in request failed: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		return nil, fmt.Errorf("account sign-in failed (HTTP %d)", resp.StatusCode)
	}
	var accountCookies []*http.Cookie
	for _, cookie := range resp.Cookies() {
		if strings.HasPrefix(cookie.Name, "aa_") {
			accountCookies = append(accountCookies, cookie)
		}
	}
	jar, _ := cookiejar.New(nil)
	jar.SetCookies(origin, accountCookies)
	cookies := jar.Cookies(origin)
	if len(cookies) == 0 {
		return nil, errors.New("account sign-in returned no session cookie; check ANNAS_SECRET_KEY")
	}
	// Cache only in memory; never log or persist credentials or cookies.
	s.entries[id] = cachedSession{jar: jar, expires: time.Now().Add(6 * time.Hour)}
	return cookies, nil
}

type sessionTransport struct {
	base     http.RoundTripper
	key      string
	sessions *sessionCache
}

func (t *sessionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	origin := &url.URL{Scheme: req.URL.Scheme, Host: req.URL.Host, Path: "/"}
	for attempt := 0; attempt < 2; attempt++ {
		request := req.Clone(req.Context())
		request.Header.Set("User-Agent", BrowserUserAgent)
		request.Header.Set("Accept-Language", "en-US,en;q=0.9")
		if t.key != "" {
			cookies, err := t.sessions.cookies(req.Context(), origin, t.key, attempt > 0, t.base)
			if err != nil {
				return nil, err
			}
			for _, cookie := range cookies {
				request.AddCookie(cookie)
			}
		}
		resp, err := t.base.RoundTrip(request)
		if err != nil {
			return nil, err
		}
		location, _ := url.Parse(resp.Header.Get("Location"))
		challenge := resp.StatusCode == http.StatusForbidden ||
			(resp.StatusCode >= 300 && resp.StatusCode < 400 && location != nil && location.Query().Get("check") == "1")
		if !challenge {
			return resp, nil
		}
		resp.Body.Close()
		if t.key == "" {
			return nil, errors.New("Anna's Archive blocked the search with a browser check; set ANNAS_SECRET_KEY to enable signed-in searches")
		}
	}
	return nil, errors.New("Anna's Archive blocked the search even after refreshing the account session")
}

func newSearchCollector(timeout time.Duration) *colly.Collector {
	c := colly.NewCollector(colly.UserAgent(BrowserUserAgent))
	c.SetClient(&http.Client{
		Timeout:   timeout,
		Transport: &sessionTransport{base: http.DefaultTransport, key: os.Getenv("ANNAS_SECRET_KEY"), sessions: &memberSessions},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Do not send account credentials to another redirect destination.
			if req.URL.Scheme != via[0].URL.Scheme || req.URL.Host != via[0].URL.Host {
				return errors.New("refusing cross-origin redirect during signed-in search")
			}
			if len(via) >= 5 {
				return errors.New("too many search redirects")
			}
			return nil
		},
	})
	return c
}
