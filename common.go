package updaterini

import (
	"errors"
	"regexp"
)

var (
	ErrorNotUniqChannelsNames = errors.New("channels names are not uniq")
)

type Channel struct {
	Name          string
	useForUpdate  bool
	weight        int
	isReleaseChan bool
}

func (ch *Channel) setUseForUpdate(use bool) {
	ch.useForUpdate = use
}

func (ch *Channel) isReleaseChannel() bool {
	return ch.isReleaseChan
}

func GetDevChanel(useForUpdate bool) Channel {
	return Channel{
		Name:         "dev",
		useForUpdate: useForUpdate,
	}
}

func GetAlphaChanel(useForUpdate bool) Channel {
	return Channel{
		Name:         "alpha",
		useForUpdate: useForUpdate,
	}
}

func GetBetaChanel(useForUpdate bool) Channel {
	return Channel{
		Name:         "beta",
		useForUpdate: useForUpdate,
	}
}

func GetReleaseChanel(useForUpdate bool) Channel {
	return Channel{
		isReleaseChan: true,
		useForUpdate:  useForUpdate,
	}
}

type applicationConfig struct {
	currentVersion        versionCurrent
	channels              []Channel
	versionRegex          *regexp.Regexp
	ShowPrepareVersionErr bool
}

/*
	version - current version with channel

	channels - channels, that used in project versioning. channels ODER IS IMPORTANT. low index more priority
*/
func NewApplicationConfig(version string, channels []Channel) (*applicationConfig, error) {
	cfg := applicationConfig{
		channels: channels,
	}
	channelsLen := len(cfg.channels)
	channelsUniqNames := make(map[string]interface{}, channelsLen)
	for i, channel := range cfg.channels {
		if _, ok := channelsUniqNames[channel.Name]; ok {
			return nil, ErrorNotUniqChannelsNames
		}
		channelsUniqNames[channel.Name] = struct{}{}
		cfg.channels[i].weight = channelsLen - i
	}
	curVersion, err := newVersionCurrent(cfg, version)
	if err != nil {
		return nil, err
	}
	if curVersion != nil {
		cfg.currentVersion = *curVersion
	}
	return &cfg, err
}

func (ac *applicationConfig) isReleaseChannelOnlyMod() bool {
	if len(ac.channels) == 1 && ac.channels[0].isReleaseChannel() {
		return true
	}
	isReleaseChannelForUpdate := false
	forUpdateChannelCounter := 0
	for _, channel := range ac.channels {
		if channel.isReleaseChannel() {
			if !channel.useForUpdate {
				return false
			}
			isReleaseChannelForUpdate = channel.useForUpdate
			continue
		}
		if channel.useForUpdate {
			forUpdateChannelCounter++
		}
	}
	return isReleaseChannelForUpdate && forUpdateChannelCounter == 0
}

func (ac *applicationConfig) isReleaseChannelAvailable() bool {
	for _, channel := range ac.channels {
		if channel.isReleaseChannel() {
			return true
		}
	}
	return false
}

func (ac *applicationConfig) getReleaseChannel() *Channel {
	for _, channel := range ac.channels {
		if channel.isReleaseChannel() {
			return &channel
		}
	}
	return nil
}

type UpdateConfig struct {
	ApplicationConfig applicationConfig
	Sources           []UpdateSource
}
