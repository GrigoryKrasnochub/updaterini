package updaterini

import (
	"regexp"
)

type Channel struct {
	Name   string // for searching in update files name
	Weight int
}

const (
	releaseChanelName   = ""
	releaseChanelWeight = 1000
)

func (ch *Channel) isReleaseChanel() bool {
	return ch.Name == releaseChanelName && ch.Weight == releaseChanelWeight
}

func GetDevChanel() Channel {
	return Channel{
		Name:   "dev",
		Weight: 0,
	}
}

func GetAlphaChanel() Channel {
	return Channel{
		Name:   "alpha",
		Weight: 1,
	}
}

func GetBetaChanel() Channel {
	return Channel{
		Name:   "beta",
		Weight: 2,
	}
}

func GetReleaseChanel() Channel {
	return Channel{
		Name:   releaseChanelName,
		Weight: releaseChanelWeight,
	}
}

type applicationConfig struct {
	currentVersion versionCurrent
	channels       []Channel
	versionRegex   *regexp.Regexp
}

func NewApplicationConfig(version string, channels []Channel) (*applicationConfig, error) {
	appConf := applicationConfig{
		channels: channels,
	}
	curVersion, err := newVersionCurrent(appConf, version)
	if err != nil {
		return nil, err
	}
	if curVersion != nil {
		appConf.currentVersion = *curVersion
	}
	return &appConf, err
}

func (ac *applicationConfig) isReleaseChanelOnlyMod() bool {
	return len(ac.channels) == 1 && ac.channels[0].isReleaseChanel()
}

func (ac *applicationConfig) isReleaseChanelAvailable() bool {
	for _, channel := range ac.channels {
		if channel.isReleaseChanel() {
			return true
		}
	}
	return false
}

type UpdateConfig struct {
	ApplicationConfig applicationConfig
	Sources           []UpdateSource
}
