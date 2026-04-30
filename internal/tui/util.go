package tui

func wrapText(text string, width int) []string {
	if width <= 0 || len(text) <= width {
		return []string{text}
	}
	var lines []string
	for len(text) > width {
		cut := width
		for cut > 0 && text[cut-1] != ' ' {
			cut--
		}
		if cut == 0 {
			cut = width
			lines = append(lines, text[:cut])
			text = text[cut:]
		} else {
			lines = append(lines, text[:cut-1])
			text = text[cut:]
		}
	}
	if len(text) > 0 {
		lines = append(lines, text)
	}
	return lines
}
