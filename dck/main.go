// Package threeddoc implements the TCB 3D DOC demo.
package threeddoc

import (
	originalassets "3d_doc"
	"bytes"
	"fmt"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/presets"

	"github.com/olivierh59500/democonstructionkit/sound"

	"image"
	"image/color"

	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/democonstructionkit/timeline"

	_ "image/png"
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	audio "github.com/olivierh59500/democonstructionkit/sound/output"
)

const (
	screenWidth  = 768
	screenHeight = 540
	fontWidth    = 62
	fontHeight   = 50
	glyphCount   = 59
	scrollerRows = 25
	sampleRate   = 44100
)

var assets = originalassets.
	DCKAssetAssets()

// Game owns the production scene state
type Game struct {
	introScroll, mainScroll *scrolling.Scrolling
	// Images
	backdrop  *ebiten.Image
	mountains *ebiten.Image
	sphere    *ebiten.Image
	shadows   [4]*ebiten.Image

	// Working canvases
	checkerboard *effects.PerspectiveCheckerboard
	ballTrain    *effects.ProjectedBallTrain
	theCanvas    *ebiten.Image

	// Scrolltext
	text1 string
	text2 string

	// 3D Doc animation
	elapsedSeconds float64

	// Audio
	audioContext *audio.Context
	audioPlayer  *audio.Player
	musicStream  *sound.Stream
	audioReady   bool

	// Fixed demo surface centered on wide displays.
	sceneCanvas *ebiten.Image

	// Scene phase and handoff boundary.
	handoff *timeline.IntroHandoff
}

// NewGame constructs an independent game instance
func NewGame() *Game {
	handoff, err := timeline.NewIntroHandoff(timeline.IntroHandoffConfig{FadeStart: 1, FadeMax: 1})
	if err != nil {
		panic(err)
	}
	g := &Game{handoff: handoff}

	// Messages
	g.text1 = "               BILIZIR FROM DMA HAVE DONE IT AGAIN: A NEW GOLANG/EBITEN CONVERSION, THIS TIME THIS IS THE 3D-DOC FROM TCB    \\          "
	g.text2 = "                          BILIZIR IS PROUD TO PRESENT THE CONVERSION OF THE 3D-DOC DEMO!    THIS SCREEN WAS ORIGINALLY RELEASED IN TCB'S CUDDLY DEMOS ON ATARI ST A LONG TIME AGO...  HERE IT'S THE GOLANG VERSION OF THE 3D-DOC WELL IT'S A FREE ADAPTATION :)   GREETINGS TO ALL MEMBERS OF DMA AND THE UNION... LET'S WRAP!   "

	return g
}

// loadImage decodes a bundled image
func (g *Game) loadImage(path string) (*ebiten.Image, error) {
	data, err := assets.ReadFile(path)
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return ebiten.NewImageFromImage(img), nil
}

// Init constructs the graphics resources
func (g *Game) Init() error {
	var err error

	// Load images
	g.backdrop, err = g.loadImage("assets/backdrop.png")
	if err != nil {
		return fmt.Errorf("load backdrop: %w", err)
	}

	g.mountains, err = g.loadImage("assets/mountains.png")
	if err != nil {
		return fmt.Errorf("load mountains: %w", err)
	}

	introFont, err := g.loadImage("assets/kh6.png")
	if err != nil {
		return fmt.Errorf("load intro font: %w", err)
	}
	introAtlas, err := presets.FontAtlas("3d_doc-intro", introFont)
	if err != nil {
		return err
	}
	introConfig := presets.DOCIntroRowBands(introAtlas, g.text1)
	g.introScroll, err = scrolling.New(scrolling.Config{RowBands: &introConfig})
	if err != nil {
		return err
	}

	scrollerFont, err := g.loadImage("assets/font_out.png")
	if err != nil {
		return fmt.Errorf("load scroller font: %w", err)
	}
	scrollerAtlas, err := presets.FontAtlas("3d_doc", scrollerFont)
	if err != nil {
		return err
	}
	mainConfig := presets.DOCMainRowBands(scrollerAtlas, g.text2)
	g.mainScroll, err = scrolling.New(scrolling.Config{RowBands: &mainConfig})
	if err != nil {
		return err
	}

	g.sphere, err = g.loadImage("assets/ball.png")
	if err != nil {
		return fmt.Errorf("load sphere: %w", err)
	}

	// Load shadow images
	for i := 0; i < 4; i++ {
		g.shadows[i], err = g.loadImage(fmt.Sprintf("assets/shadow%d.png", i+1))
		if err != nil {
			return fmt.Errorf("load shadow%d: %w", i+1, err)
		}
	}
	g.ballTrain, err = effects.NewProjectedBallTrain(presets.DOCProjectedBalls(g.sphere, g.shadows[:]))
	if err != nil {
		return err
	}

	// Construct working canvases
	g.checkerboard, err = effects.NewPerspectiveCheckerboard(presets.DOCCheckerboard())
	if err != nil {
		return err
	}
	g.theCanvas = ebiten.NewImage(384, 270)

	g.sceneCanvas = ebiten.NewImage(screenWidth, screenHeight)

	return nil
}

// initAudio opens output after the Ebitengine loop becomes active.
// On Android, the native context is not ready during mobile.SetGame.
func (g *Game) initAudio() error {
	g.audioContext = audio.NewContext(sampleRate)

	musicData, err := assets.ReadFile("assets/music.ym")
	if err != nil {
		return fmt.Errorf("music not found: %w", err)
	}

	g.musicStream, err = sound.Open("music.ym", musicData, sound.Options{SampleRate: sampleRate, Loop: true, PCMFormat: sound.PCM16, Gain: 0.5})
	if err != nil {
		return fmt.Errorf("open music: %w", err)
	}

	g.audioPlayer, err = g.audioContext.NewPlayer(g.musicStream)
	if err != nil {
		_ = g.musicStream.Close()
		g.musicStream = nil
		return fmt.Errorf("create audio player: %w", err)
	}

	g.audioPlayer.Play()

	return nil
}

func (g *Game) updateMainAnimation() error {
	if err := g.checkerboard.Update(kit.Frame{}); err != nil {
		return err
	}
	if err := g.mainScroll.Update(kit.Frame{}); err != nil {
		return err
	}
	return g.ballTrain.AdvanceAt(g.elapsedSeconds)
}

// Update advances the scene once per simulation tick
func (g *Game) Update() error {
	if !g.audioReady {
		g.audioReady = true
		if err := g.initAudio(); err != nil {
			// Audio is optional; the visual scene must keep running.
			log.Printf("audio disabled: %v", err)
		}
	}

	// Adjust volume with the up and down keys
	if g.musicStream != nil {
		if ebiten.IsKeyPressed(ebiten.KeyUp) {
			vol := g.musicStream.Volume() + 0.01
			if vol > 1.0 {
				vol = 1.0
			}
			g.musicStream.SetVolume(vol)
		}
		if ebiten.IsKeyPressed(ebiten.KeyDown) {
			vol := g.musicStream.Volume() - 0.01
			if vol < 0 {
				vol = 0
			}
			g.musicStream.SetVolume(vol)
		}
	}
	g.elapsedSeconds += 1.0 / ebiten.DefaultTPS

	if !g.handoff.Main() {
		if g.introScroll.CursorRune() == '\\' {
			g.handoff.Step(true)
			if err := g.ballTrain.PoseAt(g.elapsedSeconds); err != nil {
				return err
			}
		} else {
			g.handoff.Step(false)
		}
		if err := g.introScroll.Update(kit.Frame{}); err != nil {
			return err
		}
	} else {
		g.handoff.Step(false)
		if err := g.updateMainAnimation(); err != nil {
			return err
		}
	}

	return nil
}

// Draw renders the current scene
func (g *Game) Draw(screen *ebiten.Image) {
	scene := g.sceneCanvas
	scene.Fill(color.Black)

	if !g.handoff.Main() {
		g.introScroll.Draw(scene)
	} else {
		// Main scene

		// 1. Draw the background at its authored scale
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(77, 1)
		scene.DrawImage(g.backdrop, op)

		// 2. Draw the mountains
		scene.DrawImage(g.mountains, nil)

		// 3. Prepare the checkerboard on its working surface
		g.theCanvas.Clear()
		g.checkerboard.Draw(g.theCanvas)

		// 4. Composite the floor with the authored transform
		op = &ebiten.DrawImageOptions{}
		op.GeoM.Scale(2, 2.6)      // Enlarge the authored stage.
		op.GeoM.Translate(0, -128) // Move the projected floor into place.
		scene.DrawImage(g.theCanvas, op)

		// 5. Draw the shared animated scroller
		g.mainScroll.Draw(scene)

		// 6. Draw the projected balls last
		g.ballTrain.Draw(scene)
	}

	screen.Fill(color.Black)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64((screen.Bounds().Dx()-screenWidth)/2), 0)
	screen.DrawImage(scene, op)
}

// Layout selects the logical screen size
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return logicalWidth(outsideWidth, outsideHeight), screenHeight
}

func logicalWidth(outsideWidth, outsideHeight int) int {
	if outsideWidth <= 0 || outsideHeight <= 0 {
		return screenWidth
	}

	width := (outsideWidth*screenHeight + outsideHeight - 1) / outsideHeight
	if width < screenWidth {
		return screenWidth
	}
	if width > 1280 {
		return 1280
	}
	return width
}

// Cleanup releases owned resources
func (g *Game) Cleanup() {
	if g.introScroll != nil {
		g.introScroll.Close()
	}
	if g.mainScroll != nil {
		g.mainScroll.Close()
	}
	if g.audioPlayer != nil {
		g.audioPlayer.Close()
		g.audioPlayer = nil
	}
	if g.musicStream != nil {
		g.musicStream.Close()
		g.musicStream = nil
	}
}
