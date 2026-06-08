//go:build linux

package media

func platformSources() []Source {
	return []Source{newMPRISSource()}
}
