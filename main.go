package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	screenWidth  = 768
	screenHeight = 540
	fontWidth    = 62
	fontHeight   = 50
)

//go:embed assets/*
var assets embed.FS

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
	// Images
	backdrop  *ebiten.Image
	mountains *ebiten.Image
	font1     *ebiten.Image
	fontIn    *ebiten.Image
	fontOut   *ebiten.Image
	sphere    *ebiten.Image
	shadows   [4]*ebiten.Image

	// Canvas virtuels
	chessboard     *ebiten.Image
	chessboardMask *ebiten.Image
	scrollCanvas1  *ebiten.Image
	scrollCanvas2  *ebiten.Image
	scrollCanvas3  *ebiten.Image
	scrollCanvas4  *ebiten.Image
	scrollCanvas5  *ebiten.Image

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
	scrollX3 float64

	// 3D Doc animation
	currentRadians             float64
	overWriteFirstTwoWaveforms bool
	startTime                  time.Time

	// Audio
	audioContext *audio.Context
	audioPlayer  *audio.Player

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
		startTime:                  time.Now(),
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
		return fmt.Errorf("failed to load backdrop: %v", err)
	}

	g.mountains, err = g.loadImage("assets/mountains.png")
	if err != nil {
		return fmt.Errorf("failed to load mountains: %v", err)
	}

	g.font1, err = g.loadImage("assets/kh6.png")
	if err != nil {
		return fmt.Errorf("failed to load font1: %v", err)
	}

	g.fontIn, err = g.loadImage("assets/font_in.png")
	if err != nil {
		return fmt.Errorf("failed to load fontIn: %v", err)
	}

	g.fontOut, err = g.loadImage("assets/font_out.png")
	if err != nil {
		return fmt.Errorf("failed to load fontOut: %v", err)
	}

	g.sphere, err = g.loadImage("assets/ball.png")
	if err != nil {
		return fmt.Errorf("failed to load sphere: %v", err)
	}

	// Charger les ombres
	for i := 0; i < 4; i++ {
		g.shadows[i], err = g.loadImage(fmt.Sprintf("assets/shadow%d.png", i+1))
		if err != nil {
			return fmt.Errorf("failed to load shadow%d: %v", i+1, err)
		}
	}

	// Créer les canvas virtuels
	g.chessboard = ebiten.NewImage(1280, 80)
	g.chessboardMask = ebiten.NewImage(1280, 80)
	g.scrollCanvas1 = ebiten.NewImage(768, 50)
	g.scrollCanvas2 = ebiten.NewImage(1024, 50)  // Plus large pour les déformations
	g.scrollCanvas3 = ebiten.NewImage(1024, 50)  // Plus large pour les déformations
	g.scrollCanvas4 = ebiten.NewImage(1024, 50)  // Plus large pour les déformations
	g.scrollCanvas5 = ebiten.NewImage(1024, 120) // Plus large pour les déformations

	// Précalculer les valeurs de scroll
	g.precalcScrollX()

	// Initialiser l'audio
	g.audioContext = audio.NewContext(44100)

	// Charger la musique MP3
	musicData, err := assets.ReadFile("assets/music.mp3")
	if err != nil {
		fmt.Printf("Music not found (optional): %v\n", err)
	} else {
		musicReader := bytes.NewReader(musicData)
		decodedMusic, err := mp3.DecodeWithSampleRate(44100, musicReader)
		if err != nil {
			return fmt.Errorf("failed to decode music: %v", err)
		}

		loop := audio.NewInfiniteLoop(decodedMusic, decodedMusic.Length())
		g.audioPlayer, err = g.audioContext.NewPlayer(loop)
		if err != nil {
			return fmt.Errorf("failed to create audio player: %v", err)
		}

		g.audioPlayer.Play()
	}

	return nil
}

// drawChar dessine un caractère de la font
func (g *Game) drawChar(dst *ebiten.Image, font *ebiten.Image, char byte, x, y float64, scale float64) {
	index := 0

	switch char {
	case 32:
		index = 0
	case 33:
		index = 1
	case 39:
		index = 7
	case 40:
		index = 8
	case 41:
		index = 9
	case 44:
		index = 12
	case 45:
		index = 13
	case 46:
		index = 14
	case 48:
		index = 16
	case 49:
		index = 17
	case 50:
		index = 18
	case 51:
		index = 19
	case 52:
		index = 20
	case 53:
		index = 21
	case 54:
		index = 22
	case 55:
		index = 23
	case 56:
		index = 24
	case 57:
		index = 25
	case 58:
		index = 26
	case 59:
		index = 27
	case 63:
		index = 31
	case 65, 97:
		index = 33
	case 66, 98:
		index = 34
	case 67, 99:
		index = 35
	case 68, 100:
		index = 36
	case 69, 101:
		index = 37
	case 70, 102:
		index = 38
	case 71, 103:
		index = 39
	case 72, 104:
		index = 40
	case 73, 105:
		index = 41
	case 74, 106:
		index = 42
	case 75, 107:
		index = 43
	case 76, 108:
		index = 44
	case 77, 109:
		index = 45
	case 78, 110:
		index = 46
	case 79, 111:
		index = 47
	case 80, 112:
		index = 48
	case 81, 113:
		index = 49
	case 82, 114:
		index = 50
	case 83, 115:
		index = 51
	case 84, 116:
		index = 52
	case 85, 117:
		index = 53
	case 86, 118:
		index = 54
	case 87, 119:
		index = 55
	case 88, 120:
		index = 56
	case 89, 121:
		index = 57
	case 90, 122:
		index = 58
	default:
		index = 0
	}

	cols := 10
	srcX := (index % cols) * fontWidth
	srcY := (index / cols) * fontHeight

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(x, y)

	charImg := font.SubImage(image.Rect(srcX, srcY, srcX+fontWidth, srcY+fontHeight)).(*ebiten.Image)
	dst.DrawImage(charImg, op)
}

// drawScrollText dessine un texte défilant
func (g *Game) drawScrollText(dst *ebiten.Image, font *ebiten.Image, text string, scrollX float64) float64 {
	charSpacing := float64(fontWidth)
	startChar := int(scrollX / charSpacing)
	offset := math.Mod(scrollX, charSpacing)

	// Calculer combien de caractères on peut afficher sur toute la largeur
	maxChars := int(float64(dst.Bounds().Dx())/charSpacing) + 3

	for i := 0; i < maxChars; i++ {
		charIndex := (startChar + i) % len(text)
		if charIndex < 0 {
			charIndex += len(text)
		}

		x := float64(i)*charSpacing - offset
		if x >= -charSpacing && x < float64(dst.Bounds().Dx())+charSpacing {
			g.drawChar(dst, font, text[charIndex], x, 0, 1)
		}
	}

	// Vitesse de défilement
	return math.Mod(scrollX+3, float64(len(text))*charSpacing)
}

// drawScroller dessine le scroller avec effets
func (g *Game) drawScroller(screen *ebiten.Image) {
	// Clear canvases
	g.scrollCanvas2.Clear()
	g.scrollCanvas3.Clear()
	g.scrollCanvas5.Clear()

	// Dessiner le texte sur le canvas élargi
	g.scrollX2 = g.drawScrollText(g.scrollCanvas2, g.fontOut, g.text2, g.scrollX2)

	// Effet de vague sur le scroller
	for j := 0; j < 25; j++ {
		srcRect := image.Rect(0, j*2, 1024, (j+1)*2)
		dstX := g.scrollX[(g.vbl3+j)%g.scrollXMod]

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(dstX, float64(j*2))
		g.scrollCanvas3.DrawImage(g.scrollCanvas2.SubImage(srcRect).(*ebiten.Image), op)
	}

	// Effet de rebond vertical
	// yOffset varie de 0 à 60 (30 + 30*cos)
	yOffset := 30 + 30*math.Cos(g.vbl4/20)

	// On dessine le scroller avec un décalage vertical
	for j := 0; j < 25; j++ {
		srcRect := image.Rect(0, j*2, 1024, (j+1)*2)
		dstX := g.scrollX[(g.vbl3+j)%g.scrollXMod]

		// Position verticale avec l'effet de rebond
		dstY := float64(j*2) + yOffset

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(dstX, dstY)
		g.scrollCanvas5.DrawImage(g.scrollCanvas3.SubImage(srcRect).(*ebiten.Image), op)
	}

	// Extraire la partie visible centrée et dessiner directement
	offsetX := (1024 - 768) / 2
	visibleRect := image.Rect(offsetX, 0, offsetX+768, 120)

	// Dessiner le résultat final directement sur l'écran
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, 62)
	screen.DrawImage(g.scrollCanvas5.SubImage(visibleRect).(*ebiten.Image), op)

	g.vbl4 += 1.2
	g.vbl3++
}

// drawQuad dessine un quadrilatère rempli
func drawQuad(img *ebiten.Image, x1, y1, x2, y2, x3, y3, x4, y4 float64, c color.Color) {
	vertices := []ebiten.Vertex{
		{
			DstX:   float32(x1),
			DstY:   float32(y1),
			SrcX:   0,
			SrcY:   0,
			ColorR: float32(c.(color.RGBA).R) / 255,
			ColorG: float32(c.(color.RGBA).G) / 255,
			ColorB: float32(c.(color.RGBA).B) / 255,
			ColorA: float32(c.(color.RGBA).A) / 255,
		},
		{
			DstX:   float32(x2),
			DstY:   float32(y2),
			SrcX:   0,
			SrcY:   0,
			ColorR: float32(c.(color.RGBA).R) / 255,
			ColorG: float32(c.(color.RGBA).G) / 255,
			ColorB: float32(c.(color.RGBA).B) / 255,
			ColorA: float32(c.(color.RGBA).A) / 255,
		},
		{
			DstX:   float32(x3),
			DstY:   float32(y3),
			SrcX:   0,
			SrcY:   0,
			ColorR: float32(c.(color.RGBA).R) / 255,
			ColorG: float32(c.(color.RGBA).G) / 255,
			ColorB: float32(c.(color.RGBA).B) / 255,
			ColorA: float32(c.(color.RGBA).A) / 255,
		},
		{
			DstX:   float32(x4),
			DstY:   float32(y4),
			SrcX:   0,
			SrcY:   0,
			ColorR: float32(c.(color.RGBA).R) / 255,
			ColorG: float32(c.(color.RGBA).G) / 255,
			ColorB: float32(c.(color.RGBA).B) / 255,
			ColorA: float32(c.(color.RGBA).A) / 255,
		},
	}

	indices := []uint16{0, 1, 2, 2, 3, 0}

	op := &ebiten.DrawTrianglesOptions{}
	op.FillRule = ebiten.FillAll

	white := ebiten.NewImage(1, 1)
	white.Fill(color.White)

	img.DrawTriangles(vertices, indices, white, op)
}

// drawChessboard dessine le damier avec perspective
func (g *Game) drawChessboard() {
	g.chessboard.Clear()

	g.xMove += g.xm * g.speed * 0.005
	if g.xMove > 32 {
		g.xMove -= 32
	}
	if g.xMove < 0 {
		g.xMove += 32
	}

	chessColor := color.RGBA{96, 96, 96, 255}

	for i := -5; i < 50; i++ {
		x1 := -8 + float64(i)*32 + g.xMove
		x2 := 8 + float64(i)*32 + g.xMove
		x3 := -752 + float64(i)*192 + g.xMove*6
		x4 := -848 + float64(i)*192 + g.xMove*6

		drawQuad(g.chessboard, x1, 0, x2, 0, x3, 80, x4, 80, chessColor)
	}

	g.yMove += g.ym * g.speed * 0.016
	if g.yMove > 64 {
		g.yMove -= 64
	}
	if g.yMove < 0 {
		g.yMove += 64
	}

	g.chessboardMask.Clear()

	for i := -2; i < 8; i++ {
		y1 := -20 + (g.fov/(g.fov+float64(2*i)*32-g.yMove))*50
		y2 := -20 + (g.fov/(g.fov+float64(2*i)*32+32-g.yMove))*50

		if y1 > y2 {
			y1, y2 = y2, y1
		}

		if y2 > y1 && y1 < 80 && y2 > 0 {
			startY := math.Max(0, y1)
			endY := math.Min(80, y2)

			vector.DrawFilledRect(g.chessboardMask, 0, float32(startY), 1280, float32(endY-startY), chessColor, false)
		}
	}

	op := &ebiten.DrawImageOptions{}
	op.CompositeMode = ebiten.CompositeModeXor
	g.chessboard.DrawImage(g.chessboardMask, op)
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

// drawDoc dessine les sphères 3D animées
func (g *Game) drawDoc(screen *ebiten.Image) {
	const (
		FOCAL_LENGTH  = 400
		BALL_WIDTH    = 64
		BALL_HEIGHT   = 64
		SHADOW_WIDTH  = 64
		SHADOW_HEIGHT = 16
		ANIM_DURATION = 7
	)

	t := time.Since(g.startTime).Seconds()

	// Gestion de la boucle d'animation
	if g.overWriteFirstTwoWaveforms && t > ANIM_DURATION*3 {
		g.overWriteFirstTwoWaveforms = false
	}

	balls := make([]Sprite, 4)
	ballShadows := make([]Sprite, 4)

	for i := 0; i < 4; i++ {
		// Déterminer l'index d'animation actuel
		animIndex := int(t/ANIM_DURATION) % 8 // Changé de 7 à 8 pour inclure plus de variations

		// Après les 3 premières boucles, éviter les animations 0 et 1
		if !g.overWriteFirstTwoWaveforms && animIndex < 2 {
			animIndex = 2 + int(t/ANIM_DURATION)%6
		}

		// Si on est dans les 3 premières boucles et sur les animations 0 ou 1,
		// forcer l'utilisation de l'animation 7
		if g.overWriteFirstTwoWaveforms && animIndex < 2 {
			animIndex = 7
		}

		// Calculer l'alpha pour le blend entre deux animations
		// Réduire la vitesse de transition pour plus de fluidité
		alpha := math.Min(1, math.Mod(t/ANIM_DURATION, 1)*ANIM_DURATION*0.8) // Changé de 1.3 à 0.8

		// Obtenir les deux mouvements à mélanger
		a := getMovement(animIndex, t, i)
		b := getMovement(animIndex+1, t, i)
		anim := blendAnim(a, b, alpha)

		// Créer la position de base sur le cercle
		currentPos := Vec3{X: anim.RadiusFromCenterOfScreen, Y: 0, Z: 0}
		currentPos.RotateY(math.Pi * 2 / 360 * anim.BallLineDisplacement * float64(i))

		// Ajouter le déplacement vertical
		d := Vec3{X: 0, Y: anim.Displace, Z: 0}
		p := Vec3{X: currentPos.X + d.X, Y: currentPos.Y + d.Y, Z: currentPos.Z + d.Z}

		// IMPORTANT: Accumuler currentRadians AVANT de l'utiliser
		// Réduire la vitesse de rotation pour plus de fluidité
		g.currentRadians += (math.Pi * 2 / 360) * anim.SpinSpeed * 0.15 // Changé de 0.2 à 0.15
		g.currentRadians = math.Mod(g.currentRadians, math.Pi*2)
		p.RotateY(g.currentRadians)

		// Position de l'ombre (au sol)
		ps := Vec3{X: p.X, Y: 60, Z: p.Z}

		// Créer les sprites pour la boule et son ombre
		balls[i] = NewSprite(p, FOCAL_LENGTH, screenWidth, screenHeight)
		ballShadows[i] = NewSprite(ps, FOCAL_LENGTH, screenWidth, screenHeight)
	}

	// Trier par profondeur Z (plus loin en premier)
	// Créer des indices pour maintenir la correspondance boule/ombre
	indices := []int{0, 1, 2, 3}
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
			ballShadows[idx].U-SHADOW_WIDTH*0.5,
			ballShadows[idx].V-SHADOW_HEIGHT*0.5-verticalDisplace,
		)
		screen.DrawImage(g.shadows[shadowColor], op)
	}

	// Dessiner les sphères (dans l'ordre de profondeur)
	for _, idx := range indices {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(balls[idx].W, balls[idx].W)
		op.GeoM.Translate(
			balls[idx].U-BALL_WIDTH*0.5,
			balls[idx].V-BALL_HEIGHT*0.5,
		)
		screen.DrawImage(g.sphere, op)
	}
}

// Update met à jour l'état du jeu
func (g *Game) Update() error {
	if !g.jump {
		// Phase d'intro - détecter le caractère '\'
		charIndex := int(g.scrollX1 / float64(fontWidth))
		if charIndex < len(g.text1) && g.text1[charIndex] == '\\' {
			g.jump = true
		}
		g.scrollX1 = math.Mod(g.scrollX1+2, float64(len(g.text1))*float64(fontWidth))
	} else {
		// Animation principale
		g.speed = -1 * math.Cos(g.vbl/40)
		g.vbl += 0.16
		g.xm = 128 * math.Cos(g.vbl2/40)
		g.vbl2 += 0.8
	}

	return nil
}

// Draw dessine le jeu
func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.Black)

	if !g.jump {
		// Phase d'intro
		g.scrollCanvas1.Clear()
		g.scrollX1 = g.drawScrollText(g.scrollCanvas1, g.font1, g.text1, g.scrollX1)

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(0, 62)
		screen.DrawImage(g.scrollCanvas1, op)
	} else {
		// Scène principale

		// 1. Dessiner le fond avec le scale original
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(77, 1)
		screen.DrawImage(g.backdrop, op)

		// 2. Dessiner les montagnes
		screen.DrawImage(g.mountains, nil)

		// 3. Préparer le damier
		g.drawChessboard()

		// 4. Dessiner le damier
		op = &ebiten.DrawImageOptions{}
		op.GeoM.Scale(0.6, 2.6)
		op.GeoM.Translate(0, 260)
		screen.DrawImage(g.chessboard, op)

		// 5. Dessiner le scroller avec effets
		g.drawScroller(screen)

		// 6. Dessiner les sphères 3D en tout dernier
		g.drawDoc(screen)
	}
}

// Layout définit la taille de l'écran
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	game := NewGame()

	if err := game.Init(); err != nil {
		log.Fatal(err)
	}

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("TCB 3D DOC Demo - Go/Ebiten")

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
