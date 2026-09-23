package threeddoc

import (
	"io"
	"reflect"
	"testing"

	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/sound"

	"github.com/hajimehoshi/ebiten/v2"
)

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

func TestGlyphIndex(t *testing.T) {
	lookup, err := presets.TileLookup("3d_doc", false)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[byte]int{
		' ': 0,
		'!': 1,
		'0': 16,
		'9': 25,
		'A': 33,
		'Z': 58,
		'a': 33,
		'z': 58,
		'?': 31,
		'@': 0,
	}

	for char, want := range tests {
		got, ok := lookup(rune(char))
		if !ok {
			got = 0
		}
		if got != want {
			t.Errorf("glyphIndex(%q) = %d, want %d", char, got, want)
		}
	}
}

func TestMusicStreamReadSupportsPartialFrames(t *testing.T) {
	music, err := assets.ReadFile("assets/music.ym")
	if err != nil {
		t.Fatal(err)
	}
	player, err := sound.Open("music.ym", music, sound.Options{SampleRate: sampleRate, Loop: true, PCMFormat: sound.PCM16, Gain: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = player.Close() })

	if position, err := player.Seek(0, io.SeekStart); err != nil || position != 0 {
		t.Fatalf("reset stream = %d, %v", position, err)
	}

	for _, size := range []int{1, 2, 3, 5, 4097} {
		buffer := make([]byte, size)
		n, err := player.Read(buffer)
		if err != nil {
			t.Fatalf("Read with %d-byte buffer: %v", size, err)
		}
		if n != size {
			t.Fatalf("Read with %d-byte buffer returned %d bytes", size, n)
		}
	}
}

func TestMusicStreamVolumeIsClamped(t *testing.T) {
	music, err := assets.ReadFile("assets/music.ym")
	if err != nil {
		t.Fatal(err)
	}
	player, err := sound.Open("music.ym", music, sound.Options{SampleRate: sampleRate, Loop: true, PCMFormat: sound.PCM16, Gain: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	defer player.Close()
	player.SetVolume(-1)
	if got := player.Volume(); got != 0 {
		t.Fatalf("volume below range = %v, want 0", got)
	}
	player.SetVolume(2)
	if got := player.Volume(); got != 1 {
		t.Fatalf("volume above range = %v, want 1", got)
	}
}

func TestDrawDoesNotAdvanceAnimation(t *testing.T) {
	game := NewGame()
	if err := game.Init(); err != nil {
		t.Fatal(err)
	}
	game.jump = true
	if err := game.updateMainAnimation(); err != nil {
		t.Fatal(err)
	}

	type animationState struct {
		vbl, vbl2               float64
		xMove, yMove            float64
		introCursor, mainCursor rune
		currentRadians          float64
		docRadians              [4]float64
		overwriteFirstWaveforms bool
	}
	snapshot := func() animationState {
		return animationState{
			vbl: game.vbl, vbl2: game.vbl2,
			xMove: game.xMove, yMove: game.yMove,
			introCursor: game.introScroll.CursorRune(), mainCursor: game.mainScroll.CursorRune(),
			currentRadians: game.currentRadians, docRadians: game.docRadians,
			overwriteFirstWaveforms: game.overWriteFirstTwoWaveforms,
		}
	}

	want := snapshot()
	screen := ebiten.NewImage(screenWidth, screenHeight)
	game.Draw(screen)
	game.Draw(screen)
	if got := snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Draw changed animation state:\n got: %+v\nwant: %+v", got, want)
	}
}

func BenchmarkDraw(b *testing.B) {
	game := NewGame()
	if err := game.Init(); err != nil {
		b.Fatal(err)
	}
	game.jump = true
	game.updateMainAnimation()
	screen := ebiten.NewImage(screenWidth, screenHeight)

	b.ReportAllocs()
	for b.Loop() {
		game.Draw(screen)
	}
}

func BenchmarkMusicStreamRead(b *testing.B) {
	music, err := assets.ReadFile("assets/music.ym")
	if err != nil {
		b.Fatal(err)
	}
	player, err := sound.Open("music.ym", music, sound.Options{SampleRate: sampleRate, Loop: true, PCMFormat: sound.PCM16, Gain: 0.5})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = player.Close() })
	buffer := make([]byte, 4096)

	b.SetBytes(int64(len(buffer)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := player.Read(buffer); err != nil {
			b.Fatal(err)
		}
	}
}
