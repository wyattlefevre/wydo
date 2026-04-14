package shared

import (
	"strings"
	"unicode/utf8"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/lipgloss"
)

// PlaceOverlay composites fg centered on top of bg. Both strings may contain
// ANSI escape codes (as produced by lipgloss). Lines outside the fg area are
// left unchanged from bg.
func PlaceOverlay(bg, fg string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	fgWidth := 0
	for _, l := range fgLines {
		if w := lipgloss.Width(l); w > fgWidth {
			fgWidth = w
		}
	}
	fgHeight := len(fgLines)

	startX := (width - fgWidth) / 2
	startY := (height - fgHeight) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	result := make([]string, len(bgLines))
	copy(result, bgLines)

	for i, fgLine := range fgLines {
		y := startY + i
		if y < 0 || y >= len(result) {
			continue
		}

		bgLine := result[y]
		bgW := lipgloss.Width(bgLine)

		// Left portion of bg: visible chars [0, startX)
		left := ""
		if startX > 0 {
			left = xansi.Truncate(bgLine, startX, "")
		}

		// Right portion of bg: visible chars [startX+fgWidth, ...)
		right := ""
		cutAt := startX + fgWidth
		if cutAt < bgW {
			right = afterNVisible(bgLine, cutAt)
		}

		result[y] = left + fgLine + right
	}

	return strings.Join(result, "\n")
}

// afterNVisible returns the suffix of an ANSI-encoded string s starting after
// the first n visible (printable) characters. CSI escape sequences are skipped
// without counting toward n.
func afterNVisible(s string, n int) string {
	count := 0
	i := 0
	for i < len(s) {
		// CSI sequence: ESC [ ... <final-byte>
		if i+1 < len(s) && s[i] == '\x1b' && s[i+1] == '[' {
			i += 2
			for i < len(s) {
				b := s[i]
				i++
				// Final byte of CSI: 0x40–0x7E
				if b >= 0x40 && b <= 0x7E {
					break
				}
			}
			continue
		}
		// Other ESC sequences (e.g. ESC m): skip two bytes
		if s[i] == '\x1b' {
			i += 2
			continue
		}
		if count >= n {
			return s[i:]
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		count++
		i += size
	}
	return ""
}
