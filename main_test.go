package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBTC(t *testing.T) {
	r := &http.Request{}
	w := httptest.NewRecorder()
	btc(w, r, "btc")
}
