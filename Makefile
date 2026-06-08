.PHONY: linux windows deps clean

BIN     = media-rpc
WIN_BIN = media-rpc.exe
CC_WIN  = x86_64-w64-mingw32-gcc

# Linux build (requires libgtk-3-dev + libappindicator3-dev or libayatana-appindicator3-dev)
linux:
	CGO_ENABLED=1 go build -ldflags="-s -w" -o $(BIN) .

# Windows cross-compile from Linux (requires mingw-w64: pacman -S mingw-w64-gcc)
windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=$(CC_WIN) \
	go build -ldflags="-s -w -H windowsgui" -o $(WIN_BIN) .

# Fetch dependencies and tidy (run this first after cloning)
deps:
	go get fyne.io/systray
	go get github.com/godbus/dbus/v5
	go get github.com/gorilla/websocket
	go mod tidy

clean:
	rm -f $(BIN) $(WIN_BIN)
