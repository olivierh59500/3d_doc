package threeddoc

import "testing"

func legacyglyphIndex(char byte) int {
	switch {
	case char == '!':
		return 1
	case char == '\'':
		return 7
	case char == '(':
		return 8
	case char == ')':
		return 9
	case char == ',':
		return 12
	case char == '-':
		return 13
	case char == '.':
		return 14
	case char >= '0' && char <= '9':
		return 16 + int(char-'0')
	case char == ':':
		return 26
	case char == ';':
		return 27
	case char == '?':
		return 31
	case char >= 'A' && char <= 'Z':
		return 33 + int(char-'A')
	case char >= 'a' && char <= 'z':
		return 33 + int(char-'a')
	default:
		return 0
	}
}
func TestSharedglyphIndexMatchesOriginal(t *testing.T) {
	for r := 0; r < 256; r++ {
		if got, want := glyphIndex(byte(r)), legacyglyphIndex(byte(r)); got != want {
			t.Fatalf("rune %U: got %d, want %d", r, got, want)
		}
	}
}
