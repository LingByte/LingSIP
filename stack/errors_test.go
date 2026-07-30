package stack

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	cases := []error{
		ErrNilEndpoint,
		ErrEndpointNotOpen,
		ErrNilMessage,
		ErrUDPTransportNotStarted,
		ErrEmptyMessage,
		ErrEmptyMessageLines,
		ErrEmptyFirstLine,
		ErrEmptyMessageHeaders,
		ErrEmptyRAck,
		ErrRAckNeedsFields,
		ErrRAckCSeq,
	}
	for _, err := range cases {
		if !errors.Is(err, err) {
			t.Fatalf("not self-equal: %v", err)
		}
		if err.Error() == "" {
			t.Fatal("empty Error() string")
		}
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse(""); !errors.Is(err, ErrEmptyMessage) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := Parse("INVITE\r\n\r\n"); err == nil || !strings.Contains(err.Error(), "invalid request line") {
		t.Fatalf("short request: %v", err)
	}
	if _, err := Parse("SIP/2.0\r\n\r\n"); err == nil || !strings.Contains(err.Error(), "invalid response line") {
		t.Fatalf("short response: %v", err)
	}
	_, err := Parse("SIP/2.0 abc OK\r\n\r\n")
	if err == nil {
		t.Fatal("expected invalid status code")
	}
	var numErr *strconv.NumError
	if !errors.As(err, &numErr) {
		t.Fatalf("expected NumError wrap, got %T %v", err, err)
	}
}

func TestReadMessageEmptyHeaders(t *testing.T) {
	_, err := ReadMessage(bufio.NewReader(strings.NewReader("\r\n")))
	if !errors.Is(err, ErrEmptyMessageHeaders) {
		t.Fatalf("empty headers: %v", err)
	}
}

func TestParseRAckErrors(t *testing.T) {
	if _, _, _, err := ParseRAck(""); !errors.Is(err, ErrEmptyRAck) {
		t.Fatalf("empty: %v", err)
	}
	if _, _, _, err := ParseRAck("1 2"); !errors.Is(err, ErrRAckNeedsFields) {
		t.Fatalf("short: %v", err)
	}
	if _, _, _, err := ParseRAck("x 2 INVITE"); err == nil || !strings.Contains(err.Error(), "RAck rseq") {
		t.Fatalf("bad rseq: %v", err)
	}
	if _, _, _, err := ParseRAck("1 x INVITE"); !errors.Is(err, ErrRAckCSeq) {
		t.Fatalf("bad cseq: %v", err)
	}
}

func TestEndpointErrors(t *testing.T) {
	var ep *Endpoint
	if err := ep.Open(); !errors.Is(err, ErrNilEndpoint) {
		t.Fatalf("Open nil: %v", err)
	}
	if err := ep.Send(nil, nil); !errors.Is(err, ErrNilEndpoint) {
		t.Fatalf("Send nil ep: %v", err)
	}
	if err := ep.Serve(nil); !errors.Is(err, ErrNilEndpoint) {
		t.Fatalf("Serve nil ep: %v", err)
	}

	ep = NewEndpoint(EndpointConfig{Host: "127.0.0.1", Port: 0})
	if err := ep.Send(&Message{}, nil); !errors.Is(err, ErrEndpointNotOpen) {
		t.Fatalf("Send closed: %v", err)
	}
	if err := ep.Serve(context.Background()); !errors.Is(err, ErrEndpointNotOpen) {
		t.Fatalf("Serve closed: %v", err)
	}
	if err := ep.Open(); err != nil {
		t.Fatalf("open: %v", err)
	}
	defer ep.Close()
	if err := ep.Send(nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9}); !errors.Is(err, ErrNilMessage) {
		t.Fatalf("Send nil msg: %v", err)
	}
}

func TestUDPTransportNotStarted(t *testing.T) {
	var tr *UDPTransport
	_, _, err := tr.ReadFrom(context.Background(), make([]byte, 8))
	if !errors.Is(err, ErrUDPTransportNotStarted) {
		t.Fatalf("ReadFrom: %v", err)
	}
	_, err = tr.WriteTo(context.Background(), []byte("x"), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9})
	if !errors.Is(err, ErrUDPTransportNotStarted) {
		t.Fatalf("WriteTo: %v", err)
	}
}

func TestWrappedTransportErrors(t *testing.T) {
	root := io.ErrClosedPipe
	if !errors.Is(errListenUDP(root), root) {
		t.Fatal("listen wrap")
	}
	if !errors.Is(errReadUDP(root), root) {
		t.Fatal("read wrap")
	}
	if !errors.Is(errSendResponse(root), root) {
		t.Fatal("send wrap")
	}
	if !errors.Is(errRAckRSeq(root), root) {
		t.Fatal("rack rseq wrap")
	}
}
