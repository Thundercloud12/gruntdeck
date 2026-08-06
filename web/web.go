package web

import "embed"

//go:embed index.html login.html style.css app.js
var Files embed.FS
