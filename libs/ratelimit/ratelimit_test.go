package ratelimit

import (
	"testing"
	"time"
)

func TestAllowWindow(t *testing.T) {
	now := time.Unix(1000, 0)
	l := New(2, time.Minute)
	l.SetClock(func() time.Time { return now })

	if !l.Allow("a") || !l.Allow("a") {
		t.Fatal("first two events must pass")
	}
	if l.Allow("a") {
		t.Fatal("third event must be refused")
	}
	if !l.Allow("b") {
		t.Fatal("keys are independent")
	}
	now = now.Add(61 * time.Second)
	if !l.Allow("a") {
		t.Fatal("the window must slide")
	}
}
