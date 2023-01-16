package dynomux

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// serve returns a handler that sends a response with the given code.
func serve(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
	}
}

func TestServeMux_RemoveHandler(t *testing.T) {
	type test struct {
		path string
	}

	var tests []test
	endpoints := []string{"search", "dir", "file", "change", "count", "s"}
	tests = append(tests, test{path: "localho.st/api/v1/handler"})
	for _, e := range endpoints {
		for i := 200; i < 230; i++ {
			p := fmt.Sprintf("/%s/%d/", e, i)
			tests = append(tests, test{
				path: p,
			})
		}
	}

	mux := NewServeMux()

	t.Parallel()
	for _, tt := range tests {
		if err := mux.Handle(tt.path, serve(200)); err != nil {
			t.Fatal(err)
		}
	}

	for _, tt := range tests {
		h, p := mux.match(tt.path)
		if h == nil {
			t.Fatalf("got nil handler for pattern %s", tt.path)
		}
		if p != tt.path {
			t.Fatalf("pattern invalid: got %s, want %s", tt.path, p)
		}

		if err := mux.RemoveHandler(tt.path); err != nil {
			t.Fatalf("remove failed for pattern %s: %s", tt.path, err)
		}

		h, _ = mux.match(tt.path)
		if h != nil {
			t.Fatalf("got deleted handler for pattern %s", tt.path)
		}

		for _, entry := range mux.es {
			if entry.pattern == tt.path {
				t.Fatalf("got deleted entry for pattern %s", tt.path)
			}
		}
	}

	if len(mux.es) > 0 {
		t.Fatalf("slice of entries not empty: len got %d", len(mux.es))
	}
}

func BenchmarkServeMux(b *testing.B) {
	type test struct {
		path string
		code int
		req  *http.Request
	}

	var tests []test
	endpoints := []string{"search", "dir", "file", "change", "count", "s"}
	for _, e := range endpoints {
		for i := 200; i < 230; i++ {
			p := fmt.Sprintf("/%s/%d/", e, i)
			tests = append(tests, test{
				path: p,
				code: i,
				req:  &http.Request{Method: "GET", Host: "localhost", URL: &url.URL{Path: p}},
			})
		}
	}
	mux := NewServeMux()
	for _, tt := range tests {
		if err := mux.Handle(tt.path, serve(tt.code)); err != nil {
			b.Fatal(err)
		}
	}

	rw := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tt := range tests {
			*rw = httptest.ResponseRecorder{}
			h, pattern := mux.Handler(tt.req)
			h.ServeHTTP(rw, tt.req)
			if pattern != tt.path || rw.Code != tt.code {
				b.Fatalf("got %d, %q, want %d, %q", rw.Code, pattern, tt.code, tt.path)
			}
		}
	}
}

func Test_stripHostPort(t *testing.T) {
	type args struct {
		h string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"hostWithoutDotWithPort", args{"localhost:8080"}, "localhost"},
		{"hostWithoutDotWithoutPort", args{"localhost"}, "localhost"},
		{"ipWithPort", args{"127.0.0.1:8080"}, "127.0.0.1"},
		{"shortIpv6WithPort", args{"[::1]:8080"}, "::1"},
		{"fullIpv6WithPort", args{"[2001:470:26:6368:3e07:5468:fe5a:87ce]:8081"}, "2001:470:26:6368:3e07:5468:fe5a:87ce"},
		{"fullIpv6WithoutPort", args{"[2001:470:26:6368:3e07:5468:fe5a:87ce]"}, "2001:470:26:6368:3e07:5468:fe5a:87ce"},
		{"hostWithDotWithPort", args{"google.com:443"}, "google.com"},
		{"hostWithDotWithoutPort", args{"google.com"}, "google.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripHostPort(tt.args.h); got != tt.want {
				t.Errorf("stripHostPort() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Benchmark_StripHostPort(b *testing.B) {
	hosts := []string{
		"localhost:8080",
		"localhost",
		"127.0.0.1:8080",
		"[::1]:8080",
		"[::1]",
		"[2001:470:26:368:3e07:54ff:fe5a:87ee]:8081",
		"[2001:470:26:6368:3e07:5468:fe5a:87ce]",
		"google.com:443",
		"google.com",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, h := range hosts {
			StripHostPort(h)
		}
	}
}
