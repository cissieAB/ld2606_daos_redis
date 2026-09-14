package main

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsMissingIndexError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("idx:packets: no such index"), true},
		{fmt.Errorf("query live timestamp: %w", errors.New("Unknown Index name")), true},
		{errors.New("dial tcp: connection refused"), false},
	}
	for _, c := range cases {
		if got := isMissingIndexError(c.err); got != c.want {
			t.Errorf("isMissingIndexError(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
