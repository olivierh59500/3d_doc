// Command video exports the complete game canvas and its own audio.
package main

import (
	"flag"
	"log"
	"time"

	demo "3d_doc/dck"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/video"
)

func main() {
	config := video.Config{Output: "3d_doc.mp4", PosterAt: 60 * time.Second, Title: "3D DOC", Width: 768, Height: 540, FPS: 60, TPS: 60, SampleRate: 44100, Duration: 3 * time.Minute}
	config.Flags(flag.CommandLine)
	flag.Parse()
	if err := video.Run(config, func() (ebiten.Game, error) {
		game := demo.NewGame()
		if err := game.Init(); err != nil {
			return nil, err
		}
		return game, nil
	}); err != nil {
		log.Fatal(err)
	}
}
