package strings

import "unicode"

func HasNotGraphicCharacters(s string) bool {
	for _, c := range s {
		if !unicode.IsGraphic(c) {
			return true
		}
	}
	return false
}
