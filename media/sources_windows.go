//go:build windows

package media

func platformSources() []Source {
	return []Source{newSMTCSource()}
}
