package anna

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const searchFixture = `<html><div class="flex pt-3">
<a href="/md5/0123456789abcdef0123456789abcdef" class="custom-a block mr-2 sm:mr-4 hover:opacity-80">cover</a>
<div class="max-w-full"><a href="/md5/0123456789abcdef0123456789abcdef">Example Book</a>
<a href="/search?q=author"><span class="icon-[mdi--user-edit]"></span>Example Author</a>
<div class="text-gray-800">English [en] · EPUB · 0.5MB · 1900</div></div></div></html>`

func useSearchServer(t *testing.T, handler http.HandlerFunc, key string) *httptest.Server {
	t.Helper()
	s := httptest.NewTLSServer(handler)
	t.Cleanup(s.Close)
	t.Setenv("ANNAS_BASE_URL", strings.TrimPrefix(s.URL, "https://"))
	t.Setenv("ANNAS_AUTO_BASE_URL", "false")
	t.Setenv("ANNAS_SECRET_KEY", key)
	previous := http.DefaultTransport
	http.DefaultTransport = s.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	memberSessions = sessionCache{entries: make(map[[32]byte]cachedSession)}
	return s
}

func TestSignedInSearchReusesAndRefreshesSession(t *testing.T) {
	logins := 0
	expired := false
	useSearchServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/account/" {
			if r.Method != http.MethodPost || r.FormValue("key") != "test-key" {
				t.Error("login did not send the configured key in a POST form")
			}
			logins++
			http.SetCookie(w, &http.Cookie{Name: "aa_account_id2", Value: fmt.Sprint(logins), Path: "/", Secure: true})
			w.Header().Set("Location", "/account/")
			w.WriteHeader(http.StatusFound)
			return
		}
		cookie, err := r.Cookie("aa_account_id2")
		if err != nil || (expired && cookie.Value == "1") {
			http.Redirect(w, r, "/search?check=1", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, searchFixture)
	}, "test-key")
	books, err := FindBook("example", time.Second*5)
	if err != nil || len(books) != 1 {
		t.Fatalf("book search: results=%d error=%v", len(books), err)
	}
	if books[0].Title != "Example Book" || books[0].Format != "EPUB" || books[0].Authors != "Example Author" {
		t.Fatalf("unexpected book: %+v", books[0])
	}
	papers, err := FindArticle("example", time.Second*5)
	if err != nil || len(papers) != 1 || logins != 1 {
		t.Fatalf("article search/session reuse: results=%d logins=%d error=%v", len(papers), logins, err)
	}
	expired = true
	books, err = FindBook("example again", time.Second*5)
	if err != nil || len(books) != 1 || logins != 2 {
		t.Fatalf("session refresh: results=%d logins=%d error=%v", len(books), logins, err)
	}
}

func TestBlockedSearchReturnsError(t *testing.T) {
	useSearchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "DDoS-Guard")
	}, "")
	if _, err := FindBook("example", time.Second*5); err == nil || !strings.Contains(err.Error(), "browser check") {
		t.Fatalf("blocked book search returned %v", err)
	}
	if _, err := FindArticle("example", time.Second*5); err == nil {
		t.Fatal("blocked article search was reported as empty results")
	}
}

func TestInvalidKeyDoesNotExposeCredential(t *testing.T) {
	useSearchServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "login form")
	}, "secret-test-key")
	_, err := FindBook("example", time.Second*5)
	if err == nil || !strings.Contains(err.Error(), "no session cookie") || strings.Contains(err.Error(), "secret-test-key") {
		t.Fatalf("unexpected sign-in failure: %v", err)
	}
}

func TestCrossOriginSearchRedirectDoesNotReceiveCredential(t *testing.T) {
	unexpectedRequests := 0
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { unexpectedRequests++ }))
	defer other.Close()
	useSearchServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/account/" {
			http.SetCookie(w, &http.Cookie{Name: "aa_account_id2", Value: "test-session", Path: "/"})
			return
		}
		http.Redirect(w, r, other.URL, http.StatusFound)
	}, "test-key")
	_, err := FindBook("example", time.Second*5)
	if err == nil || !strings.Contains(err.Error(), "cross-origin") || unexpectedRequests != 0 {
		t.Fatalf("redirect: other requests=%d error=%v", unexpectedRequests, err)
	}
}
