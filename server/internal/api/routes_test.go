package api

import (
	"testing"

	"serverdash/internal/config"
)

func TestRoutesRegister(t *testing.T) {
	s := New(config.Config{}, nil, nil, nil, nil, nil, nil, nil)
	if handler := s.Routes(); handler == nil {
		t.Fatal("Routes returned nil handler")
	}
}
