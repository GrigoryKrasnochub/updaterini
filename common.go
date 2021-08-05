package updaterini

import (
	"errors"
	"fmt"
	"regexp"
	"runtime"
)

var (
	ErrorNotUniqChannelsNames = errors.New("channels names are not uniq")

	defaultValidFileNameRegex = regexp.MustCompile(fmt.Sprintf(".*%s_%s.*", runtime.GOOS, runtime.GOARCH))
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
	currentVersion            versionCurrent
	channels                  []Channel
	ValidateFilesNamesRegexes []*regexp.Regexp // match with any regex file is valid
	ShowPrepareVersionErr     bool             // on false block non-critical errors
}

/*
	version - current version with channel

	channels - channels, that used in project versioning. channels ODER IS IMPORTANT. low index more priority

	validateFilesNamesRegex - match with any regex file is valid, if nil .*GOOS_GOARCH.* is used
*/
func NewApplicationConfig(version string, channels []Channel, validateFilesNamesRegex []*regexp.Regexp) (*applicationConfig, error) {
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
	if validateFilesNamesRegex == nil {
		validateFilesNamesRegex = []*regexp.Regexp{defaultValidFileNameRegex}
	}
	cfg.ValidateFilesNamesRegexes = validateFilesNamesRegex
	return &cfg, nil
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
	Sources           []UpdateSource // source oder is source PRIORITY, response from first
}
