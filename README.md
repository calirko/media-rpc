# media-rpc

A tiny system-tray app that shows whatever you're listening to as a Discord
**"Listening to"** rich presence — title, artist, album art, and a live
progress bar. Works with Spotify, Tidal, and anything else that reports media
to the OS.

It reads media from the operating system (Windows SMTC / Linux MPRIS), so it
doesn't care which player you use, and it connects to Discord over the local
IPC pipe — or an [arRPC](https://github.com/OpenAsar/arrpc)/Vencord WebSocket
bridge if you run Discord in the browser.

## Features

- 🎵 Auto-detects the active media session and updates every 2 seconds
- 🖼️ Resolves album art to a public URL (iTunes lookup on Windows, MPRIS art
  on Linux) and upgrades Spotify/Tidal thumbnails to full resolution
- ⏱️ Shows elapsed/remaining time using the track's real duration
- 🖥️ Tray menu: toggle presence, cycle between players, blacklist a source,
  open config
- 🔌 Reconnects automatically whether Discord starts before or after the app

## Install / Run

1. Create a Discord application at <https://discord.com/developers/applications>
   and copy its **Application ID**.
2. Download `media-rpc.exe` from [Releases](../../releases) (Windows) or build
   from source (below).
3. Put it in its own folder and run it once — it writes a `config.json` next to
   the executable. Set `app_id` to your Application ID and restart.
4. The tray icon appears. Start playing music and your Discord status updates.

> Discord (desktop) must be running. For browser Discord, run arRPC and the app
> will use it automatically (`ws://127.0.0.1:1337`).

## Configuration

`config.json` lives next to the executable:

```json
{
  "app_id": "YOUR_DISCORD_APP_ID",
  "priority": ["tidal", "spotify"],
  "blacklist": [],
  "player_icons": {
    "spotify": "spotify",
    "tidal": "https://example.com/tidal.png"
  },
  "small_icon": "https://cdn.jsdelivr.net/gh/twitter/twemoji@14.0.2/assets/72x72/1f3b5.png"
}
```

| Field          | Meaning                                                                 |
| -------------- | ----------------------------------------------------------------------- |
| `app_id`       | Your Discord Application ID (**required**).                             |
| `priority`     | Player names, highest priority first, when several are playing at once. |
| `blacklist`    | Player names to ignore. Also settable from the tray menu.               |
| `player_icons` | Map player → small overlay icon (a Discord asset key or an HTTPS URL).  |
| `small_icon`   | Fallback small icon when a player has no specific one.                  |

## Dependencies

### Windows

**Build:** just [Go 1.22+](https://go.dev/dl/). No C compiler needed — the tray
uses pure-Go syscalls, so CGO can stay off.

**Run:** Windows 10/11 (uses the System Media Transport Controls API) with
built-in PowerShell 5.1+. Discord desktop, or arRPC for browser Discord.

### Linux

**Build:** Go 1.22+, a C compiler (`gcc`), `pkg-config`, and the tray libraries
— `CGO_ENABLED=1` is required:

```bash
# Debian/Ubuntu
sudo apt install build-essential pkg-config libgtk-3-dev libayatana-appindicator3-dev
# (older distros: libappindicator3-dev instead of libayatana-appindicator3-dev)
```

**Run:** a D-Bus session bus, an MPRIS-capable player (Spotify, Tidal-hifi,
most browsers), and the GTK3 + AppIndicator shared libraries above. Discord
desktop or arRPC.

## Building from source

```bash
git clone https://github.com/calirko/media-rpc
cd media-rpc
go mod download

# Windows (run on Windows)
CGO_ENABLED=0 go build -ldflags="-s -w -H windowsgui" -o media-rpc.exe .

# Linux
CGO_ENABLED=1 go build -ldflags="-s -w" -o media-rpc .
```

A `Makefile` (`make windows` / `make linux` / `make deps`) and helper scripts in
`scripts/` are also provided.

## License

MIT
