package transaction

// Observability hook for the transaction layer.
//
// sipnx keeps the default emitter as a no-op so the transaction package
// stays self-contained. Wire SetTransactionTimeoutHook at bootstrap when
// metrics are ready.

var onTransactionTimeout = func(method string) {}

// SetTransactionTimeoutHook lets tests or bootstrap intercept timer-B/F
// firings. Pass nil to reset to the default no-op emitter.
func SetTransactionTimeoutHook(fn func(method string)) {
	if fn == nil {
		onTransactionTimeout = func(method string) {}
		return
	}
	onTransactionTimeout = fn
}
