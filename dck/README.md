# DCK version

This directory contains the construction-kit version of 3d_doc. The original Go sources are preserved at their original paths (revision `914a84aa8543c0e34d3ac9c76a5ec5a70f93cc54`), with small asset accessors so both versions use the same embedded resources.

Run the original with `go run ./cmd/threeddoc` and this version with `go run ./dck/cmd/threeddoc` from the repository root.

The choreography and assets remain in this repository. Reusable rendering and
effects come from the published `github.com/olivierh59500/democonstructionkit`
module pinned in `go.mod`. Go downloads the dependencies automatically, including
`github.com/olivierh59500/ym-player v1.0.0` for YM playback. Second Reality retains its original ST3 music synchronization.
