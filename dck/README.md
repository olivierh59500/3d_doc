# DCK version

This directory contains the construction-kit version of 3d_doc. The original Go sources are preserved at their original paths (revision `914a84aa8543c0e34d3ac9c76a5ec5a70f93cc54`), with small asset accessors so both versions use the same embedded resources.

Run the original with `go run ./cmd/threeddoc` and this version with `go run ./dck/cmd/threeddoc` from the repository root.

The choreography and assets remain in this repository. Reusable rendering and
effects come from the published `github.com/olivierh59500/democonstructionkit`
module pinned in `go.mod`. Music is opened with `sound.Open`; DCK selects the decoder from the asset and
provides the configured stereo PCM format. The demo keeps its playback level and loop settings.

`scrolling.Config.RowBands` uses DCK's shared `composite.RowWarp` strip engine
for both ordered main-text passes. The same engine samples fractional source
columns for the paired fonts in Cuddly 3D DOC. Ten captures of this standalone
screen, including late main-scene frames, match the previous renderer pixel for
pixel.
The intro/main switch also uses `timeline.IntroHandoff` with no new music cue,
because this screen starts its soundtrack during the intro. The entry boundary
remains tied to the authored text control character.
