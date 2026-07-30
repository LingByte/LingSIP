package stack

import "testing"

func TestConstants(t *testing.T) {
	if VersionSIP20 != "SIP/2.0" {
		t.Fatal(VersionSIP20)
	}
	if TransportPrefix != "SIP/2.0/UDP" {
		t.Fatal(TransportPrefix)
	}
	if StatusOK != 200 || ReasonOK != "OK" {
		t.Fatal()
	}
	if ContentTypeSDP != "application/sdp" {
		t.Fatal()
	}
	if canonicalHeaderKey(HeaderCallID) != "call-id" {
		t.Fatal()
	}
}

func TestNewResponseMessage(t *testing.T) {
	resp := NewResponse(StatusOK, ReasonOK)
	if resp == nil || resp.IsRequest || resp.Version != VersionSIP20 {
		t.Fatalf("%+v", resp)
	}
	if resp.StatusCode != StatusOK || resp.StatusText != ReasonOK {
		t.Fatal()
	}
}

func TestResponseStatusLine(t *testing.T) {
	got := ResponseStatusLine(StatusOK, ReasonOK)
	if got != "SIP/2.0 200 OK" {
		t.Fatalf("got %q", got)
	}
}
