package application

import "testing"

func TestValidatePosterURL(t *testing.T) {
	allowed := []string{
		"https://image.tmdb.org/t/p/w500/abc.jpg",
		"https://IMAGE.TMDB.ORG/t/p/w185/def.jpg",
	}
	for _, u := range allowed {
		if err := validatePosterURL(u); err != nil {
			t.Errorf("expected %q to be allowed, got error: %v", u, err)
		}
	}

	blocked := []string{
		"http://image.tmdb.org/t/p/w500/abc.jpg",      // not https
		"https://169.254.169.254/computeMetadata/v1/", // cloud metadata
		"https://evil.com/x.jpg",                      // arbitrary host
		"https://image.tmdb.org.evil.com/x.jpg",       // suffix trick
		"https://image.tmdb.org@evil.com/x.jpg",       // userinfo trick
		"file:///etc/passwd",                          // non-http scheme
		"",                                            // empty
	}
	for _, u := range blocked {
		if err := validatePosterURL(u); err == nil {
			t.Errorf("expected %q to be blocked, but it was allowed", u)
		}
	}
}
