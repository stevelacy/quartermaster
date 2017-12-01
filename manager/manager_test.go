package manager

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInitResponse(t *testing.T) {
	ts := httptest.NewServer(Init("test-token", 250))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 404 {
		t.Fatal("GET / should return 404")
	}
}

func TestRun(t *testing.T) {
	ts := httptest.NewServer(Init("test-token", 250))
	defer ts.Close()

	res, err := http.NewRequest("POST", "/run", nil)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf(res)
}
