package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"video-gallery/internal/application"
	"video-gallery/pkg/config"
	"video-gallery/pkg/handlers"
)

func TestSecurityHeaders(t *testing.T) {
	h := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusTeapot {
		t.Errorf("wrapped handler should run, got status %d", rec.Code)
	}
	for name, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := rec.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("unexpected CSP %q", csp)
	}
}

func TestRouterRequiresSecretKey(t *testing.T) {
	gallerySvc := application.NewGalleryService(nil, "secret")
	admin := handlers.NewAdminHandlers(gallerySvc, nil, nil, "secret")
	mux := newRouter("secret", handlers.NewGalleryHandlers(gallerySvc), admin)

	tests := map[string]string{
		// Method checks run before any service is touched, so a registered
		// admin API route answers 405 to GET while an unregistered one 404s.
		"/secret/admin/api/clear-thumbnail": "/secret/admin/api/clear-thumbnail",
		"/wrong/admin/api/clear-thumbnail":  "",
		"/admin/api/clear-thumbnail":        "",
	}
	for path, wantPattern := range tests {
		_, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, path, nil))
		if wantPattern == "" {
			if pattern != "/" {
				t.Errorf("%s should fall through to the static file server, matched %q", path, pattern)
			}
		} else if pattern != wantPattern {
			t.Errorf("%s matched %q, want %q", path, pattern, wantPattern)
		}
	}
}

func TestNewStorageRepository(t *testing.T) {
	ctx := context.Background()

	repo, err := newStorageRepository(ctx, &config.Config{StorageBackend: "r2", BucketName: "b", R2AccountID: "a"})
	if err != nil || repo == nil {
		t.Errorf("r2 backend: got %v, %v", repo, err)
	}

	if _, err := newStorageRepository(ctx, &config.Config{StorageBackend: "s3"}); err == nil {
		t.Error("unknown backend should be rejected")
	}
}

func TestLoadConfigFlagsOverrideEnv(t *testing.T) {
	t.Setenv("SECRET_KEY", "env-secret")
	t.Setenv("BUCKET_NAME", "env-bucket")
	t.Setenv("PORT", "")
	t.Setenv("STORAGE_BACKEND", "")

	root := NewRootCmd()
	if err := root.ParseFlags([]string{"--secret-key", "flag-secret", "-p", "9999"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { secretKey, bucketName, portNumber, storageBackend = "", "", "", "" })

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SecretKey != "flag-secret" || cfg.Port != "9999" || cfg.BucketName != "env-bucket" {
		t.Errorf("flags should override env only where set, got %+v", cfg)
	}
}
