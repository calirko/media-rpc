package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"media-rpc/config"
	"media-rpc/discord"
	"media-rpc/media"
	"media-rpc/tray"
)

func main() {
	dir := execDir()
	cfgPath := filepath.Join(dir, "config.json")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		cfg = config.Default()
		if err := cfg.Save(cfgPath); err != nil {
			log.Println("could not write default config:", err)
		}
	}

	mgr := media.NewManager(cfg)
	rpc := discord.New(cfg)
	t := tray.New(cfg, cfgPath, mgr, rpc)

	go rpc.MaintainConnection()

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			mgr.Poll()
			info := mgr.Active()
			if info != nil && t.IsEnabled() {
				rpc.SetActivity(*info)
			} else {
				rpc.Clear()
			}
			t.Update(info)
		}
	}()

	t.Run()
}

func execDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
