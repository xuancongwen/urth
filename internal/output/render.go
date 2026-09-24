package output

import "strings"

// RenderText flattens a batch for a line-oriented client. Every message is
// expected to end with "\n"; the prompt is set off by a blank line and left
// unterminated so the cursor rests after it.
func RenderText(b Batch) string {
	var sb strings.Builder
	for _, m := range b.Messages {
		if m.Type == Prompt {
			sb.WriteString("\n")
		}
		sb.WriteString(m.Text)
	}
	if b.Color {
		return ANSI(sb.String())
	}
	return Strip(sb.String())
}
