package stack

import "testing"

func TestMethodConstants(t *testing.T) {
	methods := []string{
		MethodInvite, MethodAck, MethodBye, MethodCancel, MethodOptions, MethodRegister,
		MethodPrack, MethodSubscribe, MethodNotify, MethodPublish,
		MethodInfo, MethodRefer, MethodMessage, MethodUpdate,
	}
	for _, m := range methods {
		if m == "" {
			t.Fatal("empty method constant")
		}
		if m != stringsToUpper(m) {
			t.Fatalf("method %q should be upper case", m)
		}
	}
}

func stringsToUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - ('a' - 'A')
		}
	}
	return string(b)
}
