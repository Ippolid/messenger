package ratelimit

import "testing"

func TestLimiterUsesSeparateBurstForEachUser(t *testing.T) {
	t.Parallel()

	limiter := New()
	for range burst {
		if !limiter.Allow(1) {
			t.Fatal("лимитер преждевременно исчерпал burst")
		}
	}
	if limiter.Allow(1) {
		t.Fatal("лимитер должен отклонить запрос сверх burst")
	}
	if !limiter.Allow(2) {
		t.Fatal("у другого пользователя должен быть собственный лимит")
	}
}
