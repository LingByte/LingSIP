# lingsip

Pure SIP signaling library for Go.

lingsip implements the wire and session layers you need to build a SIP UAS/UAC:
message parse/serialize, UDP and TCP/TLS transport, RFC 3261 transactions,
dialog identifiers, SDP helpers, digest auth, and common carrier headers
(PAI, History-Info, STIR/SHAKEN, Reason, DTMF).

It does **not** implement RTP media, B2BUA bridging, dialplan, or persistence —
those stay in your application.

```text
go get github.com/LingByte/lingsip@latest
```

```go
import (
    "github.com/LingByte/lingsip/stack"
    "github.com/LingByte/lingsip/transaction"
    "github.com/LingByte/lingsip/uas"
    "github.com/LingByte/lingsip/sdp"
)
```

**Requirements:** Go 1.26+

---

## Features

| Area | What you get |
|------|----------------|
| **Stack** | SIP/2.0 parse & serialize, UDP `Endpoint`, TCP/TLS stream serving |
| **Transactions** | INVITE / non-INVITE client & server txs, CANCEL matching, ACK helpers |
| **UAS** | Typed method handlers + `AttachWithTransaction` wiring |
| **Dialog** | Call-ID / tags / early→confirmed matching |
| **SDP** | Audio offer parse, `BuildAudioAnswer` codec intersection, SDES / DTLS attrs |
| **Auth** | Digest challenge & verify (RFC 2617 / RFC 3261 §22) |
| **Headers** | PAI / Privacy, History-Info + Diversion, STIR/SHAKEN PASSporT, Reason |
| **DTMF** | RFC 2833/4733 + SIP INFO digit parsing |

---

## Packages

| Package | Role |
|---------|------|
| [`stack`](./stack) | Messages, constants, UDP endpoint, TCP/TLS |
| [`transaction`](./transaction) | RFC 3261 transactions |
| [`uas`](./uas) | UAS handlers & server-tx middleware |
| [`dialog`](./dialog) | Dialog identifiers |
| [`sdp`](./sdp) | SDP parse / generate / answer builder |
| [`session_timer`](./session_timer) | RFC 4028 Session-Expires |
| [`auth`](./auth) | Digest authentication |
| [`identity`](./identity) | RFC 3325 PAI / Privacy |
| [`historyinfo`](./historyinfo) | RFC 7044 History-Info + Diversion |
| [`stir`](./stir) | RFC 8224 STIR/SHAKEN |
| [`signaling`](./signaling) | RFC 3326 Reason / BYE classes |
| [`dtmf`](./dtmf) | Telephone-event & SIP INFO digits |

---

## Quick start

Minimal UAS (REGISTER, INVITE→200+SDP, ACK, BYE, CANCEL, MESSAGE; optional digest + TCP):

```bash
git clone https://github.com/LingByte/lingsip.git
cd lingsip
go run ./examples/uas-server -listen 0.0.0.0:5060

# optional:
#   -tcp :5060
#   -realm example.com -user alice -pass secret
#   -public-ip 203.0.113.10
#   -rtp-port 10000
```

Point a softphone at that address, or drive it from the UAC examples:

```bash
# terminal 1
go run ./examples/uas-server -listen 127.0.0.1:6050

# terminal 2
go run ./examples/uac-options -target 127.0.0.1:6050
go run ./examples/uac-message -target 127.0.0.1:6050 -body "hello"
```

Offline demos (no network):

```bash
go run ./examples/sdp-answer          # offer → BuildAudioAnswer
go run ./examples/digest-challenge    # Challenge401 + VerifyRequest
```

---

## Example: answer an INVITE with SDP

```go
offer, err := sdp.Parse(req.Body)
// err / nil offer → BuildAudioAnswer falls back to PCMA + telephone-event

body := sdp.BuildAudioAnswer(localIP, rtpPort, offer)
resp, err := uas.NewResponseWithTo(req, stack.StatusOK, stack.ReasonOK,
    body, stack.ContentTypeSDP, toWithLocalTag)
```

`SelectAnswerCodecs` intersects the offer with a PCMA → PCMU → G.722 → Opus preference and keeps telephone-event when present.

---

## Design notes

- **Signaling only.** No RTP sockets, codecs, or media engines.
- **No global mutable defaults.** Configure via structs and `context`.
- **UDP + stream transports.** Client txs: `RunInviteClient` / `RunNonInviteClient` (UDP) and `*Reliable` variants (TCP/TLS).
- **UAS wiring.** Prefer `uas.Handlers.AttachWithTransaction` so retransmits, CANCEL, and ACK are handled by `transaction.Manager`.

---

## Status

API may still change before `v1.0.0`. Pin a tagged release in production.

```bash
go get github.com/LingByte/lingsip@v0.1.0
```

---

## Development

```bash
go test ./...
go build ./examples/...
```

---

## License

See [`LICENSE`](./LICENSE).

lingsip is developed by [LingByte](https://github.com/LingByte) and used in production by [LingEchoX](https://github.com/LingByte/LingEchoX).
