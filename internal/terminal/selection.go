package terminal

import (
	"strings"
	"unicode/utf8"
)

// ExtractSelectedText extracts the plain text from selected cells in a snapshot grid.
// Trailing whitespace in selected lines is trimmed, and multi-line selections are
// joined with CRLF ("\r\n"). Wide characters and spacer cells are properly handled.
func ExtractSelectedText(cols, rows uint16, cells []Cell, text []byte) string {
	if len(cells) == 0 || int(cols)*int(rows) != len(cells) {
		return ""
	}

	// First pass: find the range of rows containing selected cells.
	minY := int(rows)
	maxY := -1
	for y := 0; y < int(rows); y++ {
		rowOffset := y * int(cols)
		for x := 0; x < int(cols); x++ {
			if cells[rowOffset+x].Selected != 0 {
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
				break
			}
		}
	}

	if maxY == -1 {
		return ""
	}

	var sb strings.Builder
	for y := minY; y <= maxY; y++ {
		rowOffset := y * int(cols)
		startX := -1
		endX := -1
		for x := 0; x < int(cols); x++ {
			if cells[rowOffset+x].Selected != 0 {
				if startX == -1 {
					startX = x
				}
				endX = x
			}
		}

		if startX == -1 {
			// Row is in between selected rows but has no selected cells (empty line)
			if y < maxY {
				sb.WriteString("\r\n")
			}
			continue
		}

		var lineBuilder strings.Builder
		skipSpacers := 0
		for x := startX; x <= endX; x++ {
			c := cells[rowOffset+x]
			if c.Selected == 0 {
				lineBuilder.WriteByte(' ')
				skipSpacers = 0
				continue
			}

			if c.TextLength > 0 {
				end := uint64(c.TextOffset) + uint64(c.TextLength)
				if end <= uint64(len(text)) {
					grapheme := string(text[c.TextOffset:uint32(end)])
					lineBuilder.WriteString(grapheme)
					r, _ := utf8.DecodeRuneInString(grapheme)
					if isWideRune(r) {
						skipSpacers = 1
					} else {
						skipSpacers = 0
					}
				}
			} else {
				if skipSpacers > 0 {
					skipSpacers--
				} else {
					lineBuilder.WriteByte(' ')
				}
			}
		}

		lineStr := lineBuilder.String()
		trimmed := strings.TrimRight(lineStr, " ")
		// If the entire row selection consisted of spaces and it's a single line selection, keep the spaces.
		if trimmed == "" && lineStr != "" && minY == maxY {
			trimmed = lineStr
		}

		sb.WriteString(trimmed)
		if y < maxY {
			sb.WriteString("\r\n")
		}
	}

	return sb.String()
}

// isWideRune reports whether r is typically displayed as a wide (2-cell) character in terminal grids.
func isWideRune(r rune) bool {
	if r < 0x1100 {
		return false
	}
	return (r >= 0x1100 && r <= 0x115F) ||
		r == 0x2329 || r == 0x232A ||
		(r >= 0x2E80 && r <= 0xA4CF && r != 0x303F) ||
		(r >= 0xAC00 && r <= 0xD7A3) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0xFE10 && r <= 0xFE19) ||
		(r >= 0xFE30 && r <= 0xFE6F) ||
		(r >= 0xFF01 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x1F300 && r <= 0x1F64F) ||
		(r >= 0x1F900 && r <= 0x1F9FF) ||
		(r >= 0x20000 && r <= 0x3FFFD)
}
