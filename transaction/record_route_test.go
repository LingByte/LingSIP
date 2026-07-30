package transaction

import (
	"testing"

	"github.com/LingByte/lingsip/stack"
)

func TestRouteHeadersForDialog_ReverseOrder(t *testing.T) {
	resp := &stack.Message{IsRequest: false, StatusCode: 200}
	resp.AddHeader(stack.HeaderRecordRoute, "<sip:a;lr>")
	resp.AddHeader(stack.HeaderRecordRoute, "<sip:b;lr>")
	got := RouteHeadersForDialog(resp)
	if len(got) != 2 || got[0] != "<sip:b;lr>" {
		t.Fatalf("got %v", got)
	}
}
