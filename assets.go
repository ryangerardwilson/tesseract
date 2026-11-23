package tesseract

import "embed"

// Assets contains the files the installed binary writes into ~/.tesseract/app.
//
//go:embed install.sh quickshell/session-picker/*.qml quickshell/space-navigator/*.qml tmux/*
//go:embed tmux/tmux.conf.d/*.conf tmux/scripts/*
var Assets embed.FS
