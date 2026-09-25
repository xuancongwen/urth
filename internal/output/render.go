package output

import "strings"

// Telnet option negotiation for hiding password input. Sending WILL ECHO
// tells the client the server will echo (so the client stops); WONT ECHO
// gives echo back. These are the only options the server ever sends.
const (
	telnetWillEcho = "\xff\xfb\x01"
	telnetWontEcho = "\xff\xfc\x01"
)

// RenderText flattens a batch for a line-oriented client. Every message is
// expected to end with "\n"; the prompt is set off by a blank line and left
// unterminated so the cursor rests after it.
func RenderText(b Batch) string {
	var sb strings.Builder
	for _, m := range b.Messages {
		switch m.Type {
		case Prompt:
			sb.WriteString("\n")
		case EchoOff:
			sb.WriteString(telnetWillEcho)
			continue
		case EchoOn:
			sb.WriteString(telnetWontEcho)
			continue
		case Reconnect, Commands:
			continue
		}
		sb.WriteString(m.Text)
	}
	if b.Color {
		return ANSI(sb.String())
	}
	return Strip(sb.String())
}
