package transaction

import (
	"strings"

	"github.com/LingByte/lingsip/stack"
)

// RouteHeadersForDialog constructs the Route header values used for subsequent in-dialog requests
// such as ACK, BYE, re-INVITE, and UPDATE, based on the Record-Route headers from the 2xx response
// that established the SIP dialog.
//
// This function implements RFC 3261 rules: the order of Record-Route headers MUST be reversed
// to form the correct Route set for in-dialog requests.
//
// Returns cleaned, non-empty Route values in reverse order of the received Record-Route headers.
func RouteHeadersForDialog(resp *stack.Message) []string {
	if resp == nil {
		return nil
	}

	// Extract all Record-Route headers from the 2xx dialog-establishing response
	rr := resp.GetHeaders(stack.HeaderRecordRoute)
	if len(rr) == 0 {
		return nil
	}

	// Reverse the Record-Route sequence to create the proper Route path (RFC 3261 requirement)
	out := make([]string, 0, len(rr))
	for i := len(rr) - 1; i >= 0; i-- {
		v := strings.TrimSpace(rr[i])
		if v != "" {
			out = append(out, v)
		}
	}

	return out
}
