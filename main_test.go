package threeddoc

import "testing"

func TestLogicalWidth(t *testing.T) {
	tests := []struct {
		name                 string
		outsideWidth, height int
		want                 int
	}{
		{name: "unknown size", want: screenWidth},
		{name: "original ratio", outsideWidth: 768, height: 540, want: 768},
		{name: "Pixel 10a landscape", outsideWidth: 2424, height: 1080, want: 1212},
		{name: "ultrawide cap", outsideWidth: 4000, height: 1000, want: 1280},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := logicalWidth(tt.outsideWidth, tt.height); got != tt.want {
				t.Fatalf("logicalWidth(%d, %d) = %d, want %d", tt.outsideWidth, tt.height, got, tt.want)
			}
		})
	}
}
