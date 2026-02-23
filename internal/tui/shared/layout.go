package shared

import "strings"

// CenterContent renders content vertically centered in the available height.
func CenterContent(content string, height int) string {
	content = strings.TrimRight(content, "\n")

	var contentLines []string
	if content != "" {
		contentLines = strings.Split(content, "\n")
	}

	if len(contentLines) >= height {
		return content
	}

	topPad := (height - len(contentLines)) / 2

	lines := make([]string, 0, height)
	for i := 0; i < topPad; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, contentLines...)
	// Fill remaining to reach height
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// CenterWithBottomHints renders content vertically centered in the available
// height, with hint text pinned to the very bottom line.
func CenterWithBottomHints(content, hints string, height int) string {
	content = strings.TrimRight(content, "\n")
	hints = strings.TrimRight(hints, "\n")

	var contentLines []string
	if content != "" {
		contentLines = strings.Split(content, "\n")
	}
	hintLines := strings.Split(hints, "\n")

	totalUsed := len(contentLines) + len(hintLines)
	if totalUsed >= height {
		if content == "" {
			return hints
		}
		return content + "\n" + hints
	}

	gap := height - totalUsed
	topPad := gap / 2
	bottomPad := gap - topPad

	lines := make([]string, 0, height)
	for i := 0; i < topPad; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, contentLines...)
	for i := 0; i < bottomPad; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, hintLines...)

	return strings.Join(lines, "\n")
}
