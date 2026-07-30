package stack

import (
	"strconv"
	"strings"
)

// ParseCSeqNum extracts and parses the numeric sequence number from a SIP CSeq header value
// Example: Input "1 INVITE" returns 1; Input "  123 BYE " returns 123; Invalid input returns 0
func ParseCSeqNum(cseq string) int {
	// Remove leading and trailing whitespace from the CSeq header string
	cseq = strings.TrimSpace(cseq)
	// Return 0 if the CSeq string is empty after trimming
	if cseq == "" {
		return 0
	}

	// Split the CSeq string into parts by whitespace (supports multiple spaces)
	parts := strings.Fields(cseq)
	// Return 0 if the split result has no valid parts
	if len(parts) < 1 {
		return 0
	}

	// Convert the first part (numeric sequence) to integer
	n, err := strconv.Atoi(parts[0])
	// Return 0 if conversion fails (non-numeric value)
	if err != nil {
		return 0
	}

	// Return the parsed numeric sequence number
	return n
}

// WithCSeqACK generates a valid SIP CSeq header value for ACK request,
// reusing the sequence number from the corresponding INVITE request
// Example: Input 1 returns "1 ACK"; Input 0 returns "1 ACK"
func WithCSeqACK(inviteCSeq int) string {
	// Use default sequence number 1 if input INVITE CSeq is invalid (<=0)
	if inviteCSeq <= 0 {
		return "1 ACK"
	}

	// Format the valid sequence number with ACK method and return
	return strconv.Itoa(inviteCSeq) + " ACK"
}
