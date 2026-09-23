package config

import (
	"errors"
	"testing"
)

// setEnv sets every variable Load reads, so ambient environment never leaks in
func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for _, name := range []string{
		"SECRET_KEY", "BUCKET_NAME", "PORT", "TMDB_API_KEY", "STORAGE_BACKEND",
		"R2_ACCOUNT_ID", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY",
	} {
		t.Setenv(name, vars[name])
	}
}

func TestLoadDefaults(t *testing.T) {
	setEnv(t, map[string]string{"SECRET_KEY": "s", "BUCKET_NAME": "b"})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := Config{SecretKey: "s", BucketName: "b", Port: "8080", StorageBackend: "gcs"}
	if *cfg != want {
		t.Errorf("got %+v, want %+v", *cfg, want)
	}
	if cfg.ServerAddress() != ":8080" {
		t.Errorf("ServerAddress() = %q", cfg.ServerAddress())
	}
}

func TestLoadR2(t *testing.T) {
	setEnv(t, map[string]string{
		"SECRET_KEY": "s", "BUCKET_NAME": "b", "PORT": "9000", "TMDB_API_KEY": "t",
		"STORAGE_BACKEND": "r2", "R2_ACCOUNT_ID": "acct", "R2_ACCESS_KEY_ID": "id", "R2_SECRET_ACCESS_KEY": "key",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := Config{
		SecretKey: "s", BucketName: "b", Port: "9000", TMDbAPIKey: "t",
		StorageBackend: "r2", R2AccountID: "acct", R2AccessKeyID: "id", R2SecretAccessKey: "key",
	}
	if *cfg != want {
		t.Errorf("got %+v, want %+v", *cfg, want)
	}
}

func TestLoadGCSIgnoresR2Vars(t *testing.T) {
	setEnv(t, map[string]string{"SECRET_KEY": "s", "BUCKET_NAME": "b", "R2_ACCOUNT_ID": "acct"})
	cfg, err := Load()
	if err != nil || cfg.R2AccountID != "" {
		t.Errorf("got %+v, %v; R2 settings should only load for the r2 backend", cfg, err)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	r2 := map[string]string{
		"SECRET_KEY": "s", "BUCKET_NAME": "b", "STORAGE_BACKEND": "r2",
		"R2_ACCOUNT_ID": "acct", "R2_ACCESS_KEY_ID": "id", "R2_SECRET_ACCESS_KEY": "key",
	}
	tests := map[string]error{
		"SECRET_KEY":           ErrSecretKeyNotSet,
		"BUCKET_NAME":          ErrBucketNameNotSet,
		"R2_ACCOUNT_ID":        ErrR2AccountIDNotSet,
		"R2_ACCESS_KEY_ID":     ErrR2AccessKeyIDNotSet,
		"R2_SECRET_ACCESS_KEY": ErrR2SecretAccessKeyNotSet,
	}
	for missing, wantErr := range tests {
		t.Run(missing, func(t *testing.T) {
			vars := map[string]string{}
			for k, v := range r2 {
				if k != missing {
					vars[k] = v
				}
			}
			setEnv(t, vars)
			if _, err := Load(); !errors.Is(err, wantErr) {
				t.Errorf("got %v, want %v", err, wantErr)
			}
		})
	}
}
