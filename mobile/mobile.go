// Package mobile exposes the game to ebitenmobile.
package mobile

import (
	enginemobile "github.com/hajimehoshi/ebiten/v2/mobile"

	threeddoc "3d_doc"
)

func init() {
	game := threeddoc.NewGame()
	if err := game.Init(); err != nil {
		panic(err)
	}
	enginemobile.SetGame(game)
}

// Dummy forces gomobile to include this package in the Android binding.
func Dummy() {}
