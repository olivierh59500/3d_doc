package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	threeddoc "3d_doc"
)

func main() {
	game := threeddoc.NewGame()
	if err := game.Init(); err != nil {
		log.Fatal(err)
	}
	defer game.Cleanup()

	ebiten.SetWindowSize(768, 540)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("TCB 3D DOC Demo - Go/Ebitengine")

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
