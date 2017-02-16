package assets

import "github.com/elazarl/go-bindata-assetfs"

func StaticAssetFs() *assetfs.AssetFS {
	return assetFS()
}
