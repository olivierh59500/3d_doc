// Package threeddoc implements the TCB 3D DOC demo.
package threeddoc

import (
	"bytes"
	"embed"
	"fmt"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"log"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/olivierh59500/ym-player/pkg/stsound"
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

//go:embed assets/backdrop.png assets/ball.png assets/font_out.png assets/kh6.png assets/mountains.png assets/music.ym assets/shadow*.png
var assets embed.FS

// YMPlayer wraps the YM player for Ebiten audio
type YMPlayer struct {
	player        *stsound.StSound
	buffer        []int16
	mutex         sync.Mutex
	pendingFrame  [4]byte
	pendingOffset int
	pendingCount  int
	loop          bool
	volume        float64
}

// NewYMPlayer creates a new YM player instance
func NewYMPlayer(data []byte, sampleRate int, loop bool) (*YMPlayer, error) {
	player := stsound.CreateWithRate(sampleRate)

	if err := player.LoadMemory(data); err != nil {
		player.Destroy()
		return nil, fmt.Errorf("failed to load YM data: %w", err)
	}

	player.SetLoopMode(loop)

	return &YMPlayer{
		player: player,
		buffer: make([]int16, 4096),
		loop:   loop,
		volume: 0.5,
	}, nil
}

// Read implements io.Reader for audio streaming
func (y *YMPlayer) Read(p []byte) (n int, err error) {
	y.mutex.Lock()
	defer y.mutex.Unlock()

	if len(p) == 0 {
		return 0, nil
	}
	if y.player == nil {
		return 0, io.ErrClosedPipe
	}

	if y.pendingCount > 0 {
		copied := copy(p, y.pendingFrame[y.pendingOffset:y.pendingOffset+y.pendingCount])
		y.pendingOffset += copied
		y.pendingCount -= copied
		n += copied
		if y.pendingCount == 0 {
			y.pendingOffset = 0
		}
		if n == len(p) {
			return n, nil
		}
	}

	for len(p)-n >= 4 {
		chunkSize := (len(p) - n) / 4
		if chunkSize > len(y.buffer) {
			chunkSize = len(y.buffer)
		}

		if !y.player.Compute(y.buffer[:chunkSize], chunkSize) {
			if !y.loop {
				clear(p[n:])
				return len(p), io.EOF
			}
		}

		for i := 0; i < chunkSize; i++ {
			sample := int16(float64(y.buffer[i]) * y.volume)
			pos := n + i*4
			low := byte(sample)
			high := byte(uint16(sample) >> 8)
			p[pos] = low
			p[pos+1] = high
			p[pos+2] = low
			p[pos+3] = high
		}

		n += chunkSize * 4
	}

	if n == len(p) {
		return n, nil
	}

	if !y.player.Compute(y.buffer[:1], 1) && !y.loop {
		clear(p[n:])
		return len(p), io.EOF
	}
	sample := int16(float64(y.buffer[0]) * y.volume)
	y.pendingFrame = [4]byte{byte(sample), byte(uint16(sample) >> 8), byte(sample), byte(uint16(sample) >> 8)}
	copied := copy(p[n:], y.pendingFrame[:])
	n += copied
	y.pendingOffset = copied
	y.pendingCount = len(y.pendingFrame) - copied

	return n, nil
}

// SetVolume sets the playback volume (0.0 to 1.0)
func (y *YMPlayer) SetVolume(volume float64) {
	y.mutex.Lock()
	defer y.mutex.Unlock()
	y.volume = max(0, min(1, volume))
}

// GetVolume returns the current volume
func (y *YMPlayer) GetVolume() float64 {
	y.mutex.Lock()
	defer y.mutex.Unlock()
	return y.volume
}

// Close releases resources
func (y *YMPlayer) Close() error {
	y.mutex.Lock()
	defer y.mutex.Unlock()

	if y.player != nil {
		y.player.Destroy()
		y.player = nil
	}
	return nil
}

// Vec3 représente un vecteur 3D
type Vec3 struct {
	X, Y, Z float64
}

// RotateY effectue une rotation autour de l'axe Y
func (v *Vec3) RotateY(r float64) {
	z2 := v.Z*math.Cos(r) - v.X*math.Sin(r)
	x2 := v.Z*math.Sin(r) + v.X*math.Cos(r)
	v.Z = z2
	v.X = x2
}

// Sprite représente un sprite projeté en 3D
type Sprite struct {
	U, V, W, Z float64
}

// NewSprite crée un sprite projeté depuis un point 3D
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

// Anim représente les paramètres d'animation
type Anim struct {
	SpinSpeed                float64
	Displace                 float64
	BallLineDisplacement     float64
	RadiusFromCenterOfScreen float64
}

// Game représente l'état du jeu
type Game struct {
	scrollPrograms map[*[glyphCount]*ebiten.Image]*scrolling.Scrolling
	// Images
	backdrop       *ebiten.Image
	mountains      *ebiten.Image
	introGlyphs    [glyphCount]*ebiten.Image
	scrollerGlyphs [glyphCount]*ebiten.Image
	sphere         *ebiten.Image
	shadows        [4]*ebiten.Image

	// Canvas virtuels
	chessboard     *ebiten.Image
	chessboardMask *ebiten.Image
	whitePixel     *ebiten.Image
	theCanvas      *ebiten.Image
	scrollCanvas1  *ebiten.Image
	scrollCanvas2  *ebiten.Image
	scrollCanvas3  *ebiten.Image
	scrollCanvas5  *ebiten.Image
	scrollRows2    [scrollerRows]*ebiten.Image
	scrollRows3    [scrollerRows]*ebiten.Image
	scrollVisible  *ebiten.Image
	quadVertices   []ebiten.Vertex
	quadIndices    []uint16

	// Variables d'animation
	vbl   float64
	vbl2  float64
	vbl3  int
	vbl4  float64
	xMove float64
	yMove float64
	xm    float64
	ym    float64
	fov   float64
	speed float64

	// Scroll precalc
	scrollX    []float64
	scrollXMod int

	// Scrolltext
	text1    string
	text2    string
	scrollX1 float64
	scrollX2 float64

	// 3D Doc animation
	currentRadians             float64
	docRadians                 [4]float64
	overWriteFirstTwoWaveforms bool
	elapsedSeconds             float64

	// Audio
	audioContext *audio.Context
	audioPlayer  *audio.Player
	ymPlayer     *YMPlayer
	audioReady   bool

	// Surface fixe de la démo, centrée dans les écrans larges.
	sceneCanvas *ebiten.Image

	// Phases
	jump bool
}

// NewGame crée une nouvelle instance du jeu
func NewGame() *Game {
	g := &Game{
		xm:                         0,
		ym:                         315,
		fov:                        250,
		speed:                      1,
		overWriteFirstTwoWaveforms: true,
	}

	// Textes
	g.text1 = "               BILIZIR FROM DMA HAVE DONE IT AGAIN: A NEW GOLANG/EBITEN CONVERSION, THIS TIME THIS IS THE 3D-DOC FROM TCB    \\          "
	g.text2 = "                          BILIZIR IS PROUD TO PRESENT THE CONVERSION OF THE 3D-DOC DEMO!    THIS SCREEN WAS ORIGINALLY RELEASED IN TCB'S CUDDLY DEMOS ON ATARI ST A LONG TIME AGO...  HERE IT'S THE GOLANG VERSION OF THE 3D-DOC WELL IT'S A FREE ADAPTATION :)   GREETINGS TO ALL MEMBERS OF DMA AND THE UNION... LET'S WRAP!   "

	return g
}

// loadImage charge une image depuis les assets
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

func splitFont(font *ebiten.Image) [glyphCount]*ebiten.Image {
	var glyphs [glyphCount]*ebiten.Image
	for index := range glyphs {
		srcX := (index % 10) * fontWidth
		srcY := (index / 10) * fontHeight
		glyphs[index] = font.SubImage(image.Rect(srcX, srcY, srcX+fontWidth, srcY+fontHeight)).(*ebiten.Image)
	}
	return glyphs
}

// precalcScrollX précalcule les valeurs de déplacement du scroll
func (g *Game) precalcScrollX() {
	g.scrollX = make([]float64, 0, 1024)

	// Premier pattern
	stp1 := 7.0 / 180.0 * math.Pi
	stp2 := 3.0 / 180.0 * math.Pi
	for i := 0; i < 389; i++ {
		g.scrollX = append(g.scrollX, 20*math.Sin(float64(i)*stp1)+30*math.Cos(float64(i)*stp2))
	}

	// Deuxième pattern
	stp1 = 8.0 / 180.0 * math.Pi
	for i := 0; i < 68; i++ {
		g.scrollX = append(g.scrollX, 30*math.Sin(float64(i)*stp1))
	}

	// Répétition du premier pattern
	stp1 = 7.0 / 180.0 * math.Pi
	stp2 = 3.0 / 180.0 * math.Pi
	for i := 0; i < 389; i++ {
		g.scrollX = append(g.scrollX, 20*math.Sin(float64(i)*stp1)+30*math.Cos(float64(i)*stp2))
	}

	// Dernier pattern
	stp1 = 8.0 / 180.0 * math.Pi
	for i := 0; i < 189; i++ {
		g.scrollX = append(g.scrollX, 30*math.Sin(float64(i)*stp1))
	}

	g.scrollXMod = len(g.scrollX)
}

// Init initialise les ressources
func (g *Game) Init() error {
	var err error

	// Charger les images
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
	g.introGlyphs = splitFont(introFont)

	scrollerFont, err := g.loadImage("assets/font_out.png")
	if err != nil {
		return fmt.Errorf("load scroller font: %w", err)
	}
	g.scrollerGlyphs = splitFont(scrollerFont)

	g.sphere, err = g.loadImage("assets/ball.png")
	if err != nil {
		return fmt.Errorf("load sphere: %w", err)
	}

	// Charger les ombres
	for i := 0; i < 4; i++ {
		g.shadows[i], err = g.loadImage(fmt.Sprintf("assets/shadow%d.png", i+1))
		if err != nil {
			return fmt.Errorf("load shadow%d: %w", i+1, err)
		}
	}

	// Créer les canvas virtuels
	g.chessboard = ebiten.NewImage(320, 80)
	g.chessboardMask = ebiten.NewImage(320, 80)
	g.whitePixel = ebiten.NewImage(1, 1)
	g.whitePixel.Fill(color.White)
	g.theCanvas = ebiten.NewImage(384, 270)
	g.scrollCanvas1 = ebiten.NewImage(768, 50)
	g.scrollCanvas2 = ebiten.NewImage(1024, 50)  // Plus large pour les déformations
	g.scrollCanvas3 = ebiten.NewImage(1024, 50)  // Plus large pour les déformations
	g.scrollCanvas5 = ebiten.NewImage(1024, 120) // Plus large pour les déformations
	for row := 0; row < scrollerRows; row++ {
		srcRect := image.Rect(0, row*2, 1024, (row+1)*2)
		g.scrollRows2[row] = g.scrollCanvas2.SubImage(srcRect).(*ebiten.Image)
		g.scrollRows3[row] = g.scrollCanvas3.SubImage(srcRect).(*ebiten.Image)
	}
	g.scrollVisible = g.scrollCanvas5.SubImage(image.Rect(128, 0, 896, 120)).(*ebiten.Image)
	g.quadVertices = make([]ebiten.Vertex, 0, 44)
	g.quadIndices = make([]uint16, 0, 66)

	// Précalculer les valeurs de scroll
	g.precalcScrollX()

	g.sceneCanvas = ebiten.NewImage(screenWidth, screenHeight)

	return nil
}

// initAudio ouvre le périphérique audio une fois la boucle Ebitengine active.
// Sur Android, le contexte natif n'est pas encore prêt pendant mobile.SetGame.
func (g *Game) initAudio() error {
	g.audioContext = audio.NewContext(sampleRate)

	musicData, err := assets.ReadFile("assets/music.ym")
	if err != nil {
		return fmt.Errorf("music not found: %w", err)
	}

	g.ymPlayer, err = NewYMPlayer(musicData, sampleRate, true)
	if err != nil {
		return fmt.Errorf("create YM player: %w", err)
	}

	g.audioPlayer, err = g.audioContext.NewPlayer(g.ymPlayer)
	if err != nil {
		_ = g.ymPlayer.Close()
		g.ymPlayer = nil
		return fmt.Errorf("create audio player: %w", err)
	}

	g.audioPlayer.Play()

	return nil
}

func glyphIndex(char byte) int {
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

// drawChar dessine un caractère de la font.
func (g *Game) drawChar(dst *ebiten.Image, glyphs *[glyphCount]*ebiten.Image, char byte, x, y, scale float64) {

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(x, y)
	dst.DrawImage(glyphs[glyphIndex(char)], op)
}

// drawScrollText dessine un texte défilant
func (g *Game) drawScrollText(dst *ebiten.Image, glyphs *[glyphCount]*ebiten.Image, text string, scrollX float64) {
	if len(text) == 0 {
		return
	}
	if g.scrollPrograms == nil {
		g.scrollPrograms = map[*[glyphCount]*ebiten.Image]*scrolling.Scrolling{}
	}
	program := g.scrollPrograms[glyphs]
	if program == nil {
		images := make([]*ebiten.Image, len(text))
		for i := range text {
			images[i] = glyphs[glyphIndex(text[i])]
		}
		var err error
		program, err = scrolling.FromImages(images, fontWidth)
		if err != nil {
			panic(err)
		}
		g.scrollPrograms[glyphs] = program
	}
	width := float64(fontWidth)
	first := int(scrollX / width)
	offset := math.Mod(scrollX, width)
	state := scrolling.IdentityState()
	state.X = -offset - float64(first)*width
	state.First = first
	state.End = first + int(float64(dst.Bounds().Dx())/width) + 3
	state.Cycle = true
	state.Map = func(s scrolling.Sample, op *ebiten.DrawImageOptions) bool {
		return s.X >= -width && s.X < float64(dst.Bounds().Dx())+width
	}
	program.DrawAt(dst, state)
}

// drawScroller dessine le scroller avec effets
func (g *Game) drawScroller(screen *ebiten.Image) {
	g.scrollCanvas2.Clear()
	g.scrollCanvas3.Clear()
	g.scrollCanvas5.Clear()
	g.drawScrollText(g.scrollCanvas2, &g.scrollerGlyphs, g.text2, g.scrollX2)
	frame := kit.Frame{Tick: uint64(g.vbl3)}
	composite.Strips{Thickness: 2, Count: scrollerRows, Map: func(i int, r image.Rectangle, f kit.Frame) composite.Strip {
		op := ebiten.DrawImageOptions{}
		op.GeoM.Translate(g.scrollX[(int(f.Tick)+i)%g.scrollXMod], float64(i*2))
		return composite.Strip{Source: g.scrollRows2[i].Bounds(), Options: op}
	}}.Draw(g.scrollCanvas3, g.scrollCanvas2, frame)
	yOffset := 30 + 30*math.Cos(g.vbl4/20)
	composite.Strips{Thickness: 2, Count: scrollerRows, Map: func(i int, r image.Rectangle, f kit.Frame) composite.Strip {
		op := ebiten.DrawImageOptions{}
		op.GeoM.Translate(g.scrollX[(int(f.Tick)+i)%g.scrollXMod], float64(i*2)+yOffset)
		return composite.Strip{Source: g.scrollRows3[i].Bounds(), Options: op}
	}}.Draw(g.scrollCanvas5, g.scrollCanvas3, frame)
	op := ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, 62)
	composite.Instance{Image: g.scrollVisible, Options: op}.Draw(screen)
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

// drawChessboard dessine le damier avec perspective
func (g *Game) drawChessboard(destinationCanvas *ebiten.Image) {
	// La couleur des bandes du damier
	chessColor := color.RGBA{R: 136, G: 0, B: 136, A: 255} // #880088

	// Vider les canvas de travail
	g.chessboard.Clear()
	g.chessboardMask.Clear()
	g.resetQuadBatch()

	// 1. Dessiner les bandes verticales sur le canvas principal du damier
	for i := 0; i < 11; i++ {
		x1 := -8 + float64(i)*32 + g.xMove
		x2 := 8 + float64(i)*32 + g.xMove
		x3 := -752 + float64(i)*192 + g.xMove*6
		x4 := -848 + float64(i)*192 + g.xMove*6
		g.appendQuad(x1, 0, x2, 0, x3, 80, x4, 80, chessColor)
	}
	g.drawQuadBatch(g.chessboard)

	// 2. Dessiner les bandes horizontales sur le masque
	g.resetQuadBatch()
	for i := -2; i < 8; i++ {
		y1 := -20 + (g.fov/(g.fov+float64(2*i)*32-g.yMove))*50
		y2 := -20 + (g.fov/(g.fov+float64(2*i)*32+32-g.yMove))*50
		g.appendQuad(0, y1, 320, y1, 320, y2, 0, y2, chessColor)
	}
	g.drawQuadBatch(g.chessboardMask)

	// 3. Appliquer le masque sur le canvas du damier avec l'opération XOR
	op := &ebiten.DrawImageOptions{}
	op.CompositeMode = ebiten.CompositeModeXor
	g.chessboard.DrawImage(g.chessboardMask, op)

	// 4. Dessiner le damier final sur le canvas de destination
	drawOp := &ebiten.DrawImageOptions{}
	drawOp.GeoM.Translate(32, 149)
	destinationCanvas.DrawImage(g.chessboard, drawOp)
}

// getMovement retourne les paramètres d'animation selon l'index
func getMovement(index int, t float64, i int) Anim {
	// Toujours éviter les animations 0 et 1 après le début
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
		// Pour les indices > 7, boucler sur les mouvements 2-7
		return getMovement(2+(index-2)%6, t, i)
	}
}

// blendAnim mélange deux animations
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

// drawDoc dessine les sphères 3D animées
func (g *Game) drawDoc(screen *ebiten.Image) {
	t := g.elapsedSeconds
	var balls [4]Sprite
	var ballShadows [4]Sprite

	for i := 0; i < 4; i++ {
		anim := g.currentMovement(t, i)

		// Créer la position de base sur le cercle
		currentPos := Vec3{X: anim.RadiusFromCenterOfScreen, Y: 0, Z: 0}
		currentPos.RotateY(math.Pi * 2 / 360 * anim.BallLineDisplacement * float64(i))

		// Ajouter le déplacement vertical
		d := Vec3{X: 0, Y: anim.Displace, Z: 0}
		p := Vec3{X: currentPos.X + d.X, Y: currentPos.Y + d.Y, Z: currentPos.Z + d.Z}

		p.RotateY(g.docRadians[i])

		// Position de l'ombre (au sol)
		ps := Vec3{X: p.X, Y: 60, Z: p.Z}

		// Créer les sprites pour la boule et son ombre
		balls[i] = NewSprite(p, focalLength, screenWidth, screenHeight)
		ballShadows[i] = NewSprite(ps, focalLength, screenWidth, screenHeight)
	}

	// Trier par profondeur Z (plus loin en premier)
	// Créer des indices pour maintenir la correspondance boule/ombre
	indices := [4]int{0, 1, 2, 3}
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 4; j++ {
			if balls[indices[i]].Z < balls[indices[j]].Z {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	// Dessiner les ombres d'abord (dans l'ordre de profondeur)
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

	// Dessiner les sphères (dans l'ordre de profondeur)
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

func advanceScroll(position, speed float64, text string) float64 {
	if len(text) == 0 {
		return 0
	}
	return wrap(position+speed, float64(len(text)*fontWidth))
}

func (g *Game) updateMainAnimation() {
	g.speed = -math.Cos(g.vbl / 40)
	g.vbl += 0.16
	g.xm = 128 * math.Cos(g.vbl2/40)
	g.vbl2 += 0.8

	g.xMove = wrap(g.xMove+g.xm*g.speed*0.01, 32)
	g.yMove = wrap(g.yMove+g.ym*g.speed*0.032, 64)
	g.scrollX2 = advanceScroll(g.scrollX2, 3, g.text2)
	g.vbl4 += 1.2
	g.vbl3 = (g.vbl3 + 1) % g.scrollXMod
	g.updateDocAnimation()
}

// Update met à jour l'état du jeu
func (g *Game) Update() error {
	if !g.audioReady {
		g.audioReady = true
		if err := g.initAudio(); err != nil {
			// La musique est facultative : la démo visuelle doit continuer.
			log.Printf("audio disabled: %v", err)
		}
	}

	// Contrôle du volume avec les touches haut/bas
	if g.ymPlayer != nil {
		if ebiten.IsKeyPressed(ebiten.KeyUp) {
			vol := g.ymPlayer.GetVolume() + 0.01
			if vol > 1.0 {
				vol = 1.0
			}
			g.ymPlayer.SetVolume(vol)
		}
		if ebiten.IsKeyPressed(ebiten.KeyDown) {
			vol := g.ymPlayer.GetVolume() - 0.01
			if vol < 0 {
				vol = 0
			}
			g.ymPlayer.SetVolume(vol)
		}
	}
	g.elapsedSeconds += 1.0 / ebiten.DefaultTPS

	if !g.jump {
		// Phase d'intro - détecter le caractère '\'
		charIndex := int(g.scrollX1 / float64(fontWidth))
		if charIndex < len(g.text1) && g.text1[charIndex] == '\\' {
			g.jump = true
		}
		// L'ancienne version avançait de 2 dans Update et de 3 dans Draw.
		// Conserver le total de 5 par tick rend le rythme indépendant du rafraîchissement.
		g.scrollX1 = advanceScroll(g.scrollX1, 5, g.text1)
	} else {
		g.updateMainAnimation()
	}

	return nil
}

// Draw dessine le jeu
func (g *Game) Draw(screen *ebiten.Image) {
	scene := g.sceneCanvas
	scene.Fill(color.Black)

	if !g.jump {
		// Phase d'intro
		g.scrollCanvas1.Clear()
		g.drawScrollText(g.scrollCanvas1, &g.introGlyphs, g.text1, g.scrollX1)

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(0, 62)
		scene.DrawImage(g.scrollCanvas1, op)
	} else {
		// Scène principale

		// 1. Dessiner le fond avec le scale original
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(77, 1)
		scene.DrawImage(g.backdrop, op)

		// 2. Dessiner les montagnes
		scene.DrawImage(g.mountains, nil)

		// 3. Préparer le damier sur le canvas intermédiaire
		g.theCanvas.Clear()
		g.drawChessboard(g.theCanvas)

		// 4. Dessiner le canvas intermédiaire sur l'écran final avec transformation
		op = &ebiten.DrawImageOptions{}
		op.GeoM.Scale(2, 2.6)      // Agrandissement
		op.GeoM.Translate(0, -128) // Décalage
		scene.DrawImage(g.theCanvas, op)

		// 5. Dessiner le scroller avec effets
		g.drawScroller(scene)

		// 6. Dessiner les sphères 3D en tout dernier
		g.drawDoc(scene)
	}

	screen.Fill(color.Black)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64((screen.Bounds().Dx()-screenWidth)/2), 0)
	screen.DrawImage(scene, op)
}

// Layout définit la taille de l'écran
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

// Cleanup nettoie les ressources
func (g *Game) Cleanup() {
	if g.audioPlayer != nil {
		g.audioPlayer.Close()
		g.audioPlayer = nil
	}
	if g.ymPlayer != nil {
		g.ymPlayer.Close()
		g.ymPlayer = nil
	}
}
