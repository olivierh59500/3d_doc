package threeddoc

import "embed"

// DCKAssetAssets shares an embedded resource with the optional DCK version.
func DCKAssetAssets() embed.FS { return assets }
