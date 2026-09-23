// Package threeddoc implements the TCB 3D DOC demo.
package threeddoc

import (
	originalassets "3d_doc"
	"bytes"
	"fmt"
	"github.com/olivierh59500/democonstructionkit/presets"

	"github.com/olivierh59500/democonstructionkit/sound"

	"image"
	"image/color"

	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/scrolling"

	_ "image/png"
	"log"
	"math"

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
	animDuration = 7.0
	focalLength  = 400.0
	ballWidth    = 64.0
	ballHeight   = 64.0
	shadowWidth  = 64.0
	shadowHeight = 16.0
)

var assets = originalassets.
	DCKAssetAssets()

// Vec3 is a three-dimensional vector
type Vec3 struct {
	X, Y, Z float64
}

// RotateY rotates around the Y axis
func (v *Vec3) RotateY(r float64) {
	z2 := v.Z*math.Cos(r) - v.X*math.Sin(r)
	x2 := v.Z*math.Sin(r) + v.X*math.Cos(r)
	v.Z = z2
	v.X = x2
}

// Sprite stores a projected three-dimensional sprite
type Sprite struct {
	U, V, W, Z float64
}

// NewSprite projects a three-dimensional point into a sprite
func NewSprite(p Vec3, focalLength float64, canvasWidth, canvasHeight int) Sprite {
	centerX := float64(canvasWidth) / 2
	centerY := float64(canvasHeight)/2 + 40

	scale := focalLength / (focalLength + p.Z)
	return Sprite{
		U: p.X*scale + centerX,
		V: p.Y*scale + centerY,
		W: scale * 0.7,
		Z: p.Z,
	}
}

// Anim stores movement parameters
type Anim struct {
	SpinSpeed                float64
	Displace                 float64
	BallLineDisplacement     float64
	RadiusFromCenterOfScreen float64
}

// Game owns the production scene state
type Game struct {
	introScroll, mainScroll *scrolling.Scrolling
	// Images
	backdrop  *ebiten.Image
	mountains *ebiten.Image
	sphere    *ebiten.Image
	shadows   [4]*ebiten.Image

	// Working canvases
	chessboard     *ebiten.Image
	chessboardMask *ebiten.Image
	whitePixel     *ebiten.Image
	theCanvas      *ebiten.Image
	quadVertices   []ebiten.Vertex
	quadIndices    []uint16

	// Animation state
	vbl   float64
	vbl2  float64
	xMove float64
	yMove float64
	xm    float64
	ym    float64
	fov   float64
	speed float64

	// Scrolltext
	text1 string
	text2 string

	// 3D Doc animation
	currentRadians             float64
	docRadians                 [4]float64
	overWriteFirstTwoWaveforms bool
	elapsedSeconds             float64

	// Audio
	audioContext *audio.Context
	audioPlayer  *audio.Player
	musicStream  *sound.Stream
	audioReady   bool

	// Fixed demo surface centered on wide displays.
	sceneCanvas *ebiten.Image

	// Scene phases
	jump bool
}

// NewGame constructs an independent game instance
func NewGame() *Game {
	g := &Game{
		xm:                         0,
		ym:                         315,
		fov:                        250,
		speed:                      1,
		overWriteFirstTwoWaveforms: true,
	}

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

	// Construct working canvases
	g.chessboard = ebiten.NewImage(320, 80)
	g.chessboardMask = ebiten.NewImage(320, 80)
	g.whitePixel = ebiten.NewImage(1, 1)
	g.whitePixel.Fill(color.White)
	g.theCanvas = ebiten.NewImage(384, 270)
	g.quadVertices = make([]ebiten.Vertex, 0, 44)
	g.quadIndices = make([]uint16, 0, 66)

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

func (g *Game) resetQuadBatch() {
	g.quadVertices = g.quadVertices[:0]
	g.quadIndices = g.quadIndices[:0]
}

func (g *Game) appendQuad(x1, y1, x2, y2, x3, y3, x4, y4 float64, c color.RGBA) {
	red := float32(c.R) / 255
	green := float32(c.G) / 255
	blue := float32(c.B) / 255
	alpha := float32(c.A) / 255
	base := uint16(len(g.quadVertices))
	g.quadVertices = append(g.quadVertices,
		ebiten.Vertex{
			DstX:   float32(x1),
			DstY:   float32(y1),
			SrcX:   0,
			SrcY:   0,
			ColorR: red,
			ColorG: green,
			ColorB: blue,
			ColorA: alpha,
		},
		ebiten.Vertex{
			DstX:   float32(x2),
			DstY:   float32(y2),
			SrcX:   0,
			SrcY:   0,
			ColorR: red,
			ColorG: green,
			ColorB: blue,
			ColorA: alpha,
		},
		ebiten.Vertex{
			DstX:   float32(x3),
			DstY:   float32(y3),
			SrcX:   0,
			SrcY:   0,
			ColorR: red,
			ColorG: green,
			ColorB: blue,
			ColorA: alpha,
		},
		ebiten.Vertex{
			DstX:   float32(x4),
			DstY:   float32(y4),
			SrcX:   0,
			SrcY:   0,
			ColorR: red,
			ColorG: green,
			ColorB: blue,
			ColorA: alpha,
		},
	)
	g.quadIndices = append(g.quadIndices, base, base+1, base+2, base+2, base+3, base)
}

func (g *Game) drawQuadBatch(destination *ebiten.Image) {
	op := &ebiten.DrawTrianglesOptions{}
	op.FillRule = ebiten.FillAll
	destination.DrawTriangles(g.quadVertices, g.quadIndices, g.whitePixel, op)
}

// drawChessboard renders the perspective checkerboard
func (g *Game) drawChessboard(destinationCanvas *ebiten.Image) {
	// Checkerboard stripe color
	chessColor := color.RGBA{R: 136, G: 0, B: 136, A: 255} // #880088

	// Clear the working surfaces
	g.chessboard.Clear()
	g.chessboardMask.Clear()
	g.resetQuadBatch()

	// 1. Draw vertical strips on the floor surface
	for i := 0; i < 11; i++ {
		x1 := -8 + float64(i)*32 + g.xMove
		x2 := 8 + float64(i)*32 + g.xMove
		x3 := -752 + float64(i)*192 + g.xMove*6
		x4 := -848 + float64(i)*192 + g.xMove*6
		g.appendQuad(x1, 0, x2, 0, x3, 80, x4, 80, chessColor)
	}
	g.drawQuadBatch(g.chessboard)

	// 2. Draw horizontal strips into the mask
	g.resetQuadBatch()
	for i := -2; i < 8; i++ {
		y1 := -20 + (g.fov/(g.fov+float64(2*i)*32-g.yMove))*50
		y2 := -20 + (g.fov/(g.fov+float64(2*i)*32+32-g.yMove))*50
		g.appendQuad(0, y1, 320, y1, 320, y2, 0, y2, chessColor)
	}
	g.drawQuadBatch(g.chessboardMask)

	// 3. Combine the mask with the floor using XOR
	op := &ebiten.DrawImageOptions{}
	op.CompositeMode = ebiten.CompositeModeXor
	g.chessboard.DrawImage(g.chessboardMask, op)

	// 4. Draw the completed floor on the destination
	drawOp := &ebiten.DrawImageOptions{}
	drawOp.GeoM.Translate(32, 149)
	destinationCanvas.DrawImage(g.chessboard, drawOp)
}

// getMovement selects an authored movement program
func getMovement(index int, t float64, i int) Anim {
	// Skip entrance programs zero and one after the opening
	if index < 2 && t > 21 { // Après 3 cycles de 7 secondes
		index = 2 + int(t/7)%6 // Boucler sur les animations 2-7
	}

	switch index {
	case 0, 1:
		return Anim{-5, 40, 0, 0}
	case 2:
		return Anim{-5, -60 - math.Sin(t*7)*95, 35, 150}
	case 3:
		return Anim{5, math.Sin((t+float64(i))*0.5*13)*90 - 50, 16, 150}
	case 4:
		return Anim{5, 80 - math.Abs(math.Sin((t+float64(i))*0.125*13.5)*8*math.Cos((t+float64(i))*0.125*13.5)*42) - 50, 20, 150}
	case 5:
		return Anim{5, math.Sin((t+float64(i))*0.25*13.5)*8*math.Cos((t+float64(i))*0.25*13.5)*22 - 50, 20, 150}
	case 6:
		return Anim{-7, math.Sin((t+float64(i))*0.25*13.5)*8*math.Cos((t+float64(i))*0.25*13.5)*22 - 50, 20, 150}
	case 7:
		return Anim{-8, 10 - math.Abs(math.Sin((t*0.6+float64(i)*0.05)*1.75)*70)*2.3, 20, 150}
	default:
		// Loop programs two through seven for later indices
		return getMovement(2+(index-2)%6, t, i)
	}
}

// blendAnim interpolates two movement poses
func blendAnim(a, b Anim, alpha float64) Anim {
	return Anim{
		SpinSpeed:                a.SpinSpeed*(1-alpha) + b.SpinSpeed*alpha,
		Displace:                 a.Displace*(1-alpha) + b.Displace*alpha,
		BallLineDisplacement:     a.BallLineDisplacement*(1-alpha) + b.BallLineDisplacement*alpha,
		RadiusFromCenterOfScreen: a.RadiusFromCenterOfScreen*(1-alpha) + b.RadiusFromCenterOfScreen*alpha,
	}
}

func (g *Game) currentMovement(t float64, ball int) Anim {
	animIndex := int(t/animDuration) % 8
	if !g.overWriteFirstTwoWaveforms && animIndex < 2 {
		animIndex = 2 + int(t/animDuration)%6
	}
	if g.overWriteFirstTwoWaveforms && animIndex < 2 {
		animIndex = 7
	}

	alpha := math.Min(1, math.Mod(t/animDuration, 1)*animDuration*0.8)
	return blendAnim(
		getMovement(animIndex, t, ball),
		getMovement(animIndex+1, t, ball),
		alpha,
	)
}

func (g *Game) updateDocAnimation() {
	if g.overWriteFirstTwoWaveforms && g.elapsedSeconds > animDuration*3 {
		g.overWriteFirstTwoWaveforms = false
	}

	for ball := range g.docRadians {
		anim := g.currentMovement(g.elapsedSeconds, ball)
		g.currentRadians += (math.Pi * 2 / 360) * anim.SpinSpeed * 0.15
		g.currentRadians = math.Mod(g.currentRadians, math.Pi*2)
		g.docRadians[ball] = g.currentRadians
	}
}

// drawDoc renders the animated projected balls
func (g *Game) drawDoc(screen *ebiten.Image) {
	t := g.elapsedSeconds
	var balls [4]Sprite
	var ballShadows [4]Sprite

	for i := 0; i < 4; i++ {
		anim := g.currentMovement(t, i)

		// Place the point on its base circle
		currentPos := Vec3{X: anim.RadiusFromCenterOfScreen, Y: 0, Z: 0}
		currentPos.RotateY(math.Pi * 2 / 360 * anim.BallLineDisplacement * float64(i))

		// Add vertical motion
		d := Vec3{X: 0, Y: anim.Displace, Z: 0}
		p := Vec3{X: currentPos.X + d.X, Y: currentPos.Y + d.Y, Z: currentPos.Z + d.Z}

		p.RotateY(g.docRadians[i])

		// Place the shadow on the floor
		ps := Vec3{X: p.X, Y: 60, Z: p.Z}

		// Build the ball and shadow sprites
		balls[i] = NewSprite(p, focalLength, screenWidth, screenHeight)
		ballShadows[i] = NewSprite(ps, focalLength, screenWidth, screenHeight)
	}

	// Sort by depth, farthest first
	// Keep matching ball and shadow indices
	indices := [4]int{0, 1, 2, 3}
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 4; j++ {
			if balls[indices[i]].Z < balls[indices[j]].Z {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	// Draw shadows first in depth order
	for _, idx := range indices {
		shadowColor := int(((ballShadows[idx].W - 0.5) * 10) / 2)
		shadowColor = 3 - max(0, min(3, shadowColor))

		verticalDisplace := math.Min(1, math.Max(0, 1-ballShadows[idx].W)) * 26

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(ballShadows[idx].W, ballShadows[idx].W)
		op.GeoM.Translate(
			ballShadows[idx].U-shadowWidth*0.5,
			ballShadows[idx].V-shadowHeight*0.5-verticalDisplace,
		)
		screen.DrawImage(g.shadows[shadowColor], op)
	}

	// Draw balls in depth order
	for _, idx := range indices {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(balls[idx].W, balls[idx].W)
		op.GeoM.Translate(
			balls[idx].U-ballWidth*0.5,
			balls[idx].V-ballHeight*0.5,
		)
		screen.DrawImage(g.sphere, op)
	}
}

func wrap(value, period float64) float64 {
	value = math.Mod(value, period)
	if value < 0 {
		value += period
	}
	return value
}

func (g *Game) updateMainAnimation() error {
	g.speed = -math.Cos(g.vbl / 40)
	g.vbl += 0.16
	g.xm = 128 * math.Cos(g.vbl2/40)
	g.vbl2 += 0.8

	g.xMove = wrap(g.xMove+g.xm*g.speed*0.01, 32)
	g.yMove = wrap(g.yMove+g.ym*g.speed*0.032, 64)
	if err := g.mainScroll.Update(kit.Frame{}); err != nil {
		return err
	}
	g.updateDocAnimation()
	return nil
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

	if !g.jump {
		if g.introScroll.CursorRune() == '\\' {
			g.jump = true
		}
		if err := g.introScroll.Update(kit.Frame{}); err != nil {
			return err
		}
	} else if err := g.updateMainAnimation(); err != nil {
		return err
	}

	return nil
}

// Draw renders the current scene
func (g *Game) Draw(screen *ebiten.Image) {
	scene := g.sceneCanvas
	scene.Fill(color.Black)

	if !g.jump {
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
		g.drawChessboard(g.theCanvas)

		// 4. Composite the floor with the authored transform
		op = &ebiten.DrawImageOptions{}
		op.GeoM.Scale(2, 2.6)      // Agrandissement
		op.GeoM.Translate(0, -128) // Décalage
		scene.DrawImage(g.theCanvas, op)

		// 5. Draw the shared animated scroller
		g.mainScroll.Draw(scene)

		// 6. Draw the projected balls last
		g.drawDoc(scene)
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
