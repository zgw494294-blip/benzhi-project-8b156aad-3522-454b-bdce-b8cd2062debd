package httpapi

import "embed"

//go:embed web/index.html web/styles.css web/features.css web/app.js
var webAssets embed.FS
