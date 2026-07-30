package transaction

import "testing"

func TestSetTransactionTimeoutHook_Reset(t *testing.T) {
	var seen string
	SetTransactionTimeoutHook(func(m string) { seen = m })
	onTransactionTimeout("UPDATE")
	if seen != "UPDATE" {
		t.Fatalf("got %q", seen)
	}
	SetTransactionTimeoutHook(nil)
	onTransactionTimeout("OPTIONS")
}
