package historyinfo

import "strings"

// BuildRetargetHeaders constructs RFC 7044 History-Info and RFC 5806 Diversion chains
// for a B2BUA-retargeted outbound INVITE.
func BuildRetargetHeaders(
	rawTo, rawHistoryInfo, rawDiversion string,
	newTargetURI string,
	historyReason string,
	diversionReason string,
) ([]Entry, []Diversion) {
	newTargetURI = strings.TrimSpace(newTargetURI)
	originalTo := ExtractURIFromToHeader(rawTo)
	if newTargetURI == "" {
		return nil, nil
	}

	inboundHistory := ParseChain(rawHistoryInfo)
	inboundDiversion := ParseDiversionChain(rawDiversion)

	if originalTo == "" && len(inboundHistory) == 0 && len(inboundDiversion) == 0 {
		return nil, nil
	}

	hi := AppendTransferEntry(inboundHistory, originalTo, newTargetURI, historyReason)
	dv := AppendDiversionEntry(inboundDiversion, originalTo, diversionReason)
	return hi, dv
}
