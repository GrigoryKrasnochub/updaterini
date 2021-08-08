package updaterini

import (
	"fmt"
	"regexp"
	"runtime"

	"github.com/blang/semver/v4"
)

var defaultValidFileNameRegex = regexp.MustCompile(fmt.Sprintf(".*%s_%s.*", runtime.GOOS, runtime.GOARCH))

type Channel struct {
	name          string
	useForUpdate  bool
	weight        int
	isReleaseChan bool
}

func NewChannel(name string, useForUpdate bool) Channel {
	return Channel{
		name:         name,
		useForUpdate: useForUpdate,
	}
}

func NewReleaseChannel(useForUpdate bool) Channel {
	return Channel{
		isReleaseChan: true,
		useForUpdate:  useForUpdate,
	}
}

type versionCurrent struct {
	channel Channel
	version semver.Version
}

func newVersionCurrent(cfg ApplicationConfig, version string) (versionCurrent, error) {
	pVersion, channel, err := parseVersion(cfg, version)
	if err != nil {
		return versionCurrent{}, err
	}
	curVer := versionCurrent{
		version: pVersion,
	}
	curVer.channel = channel
	return curVer, nil
}

type ApplicationConfig struct {
	currentVersion            versionCurrent
	channels                  []Channel
	ValidateFilesNamesRegexes []*regexp.Regexp // match with any regex file is valid
	ShowPrepareVersionErr     bool             // on false block non-critical errors
}

/*
	version - current version with channel

	channels - channels, that used in project versioning. channels ODER IS IMPORTANT. First in slice -  max priority EXCEPT Release Channel, it always has max priority

	validateFilesNamesRegex - match with any regex file is valid, if nil .*GOOS_GOARCH.* is used
*/
func NewApplicationConfig(version string, channels []Channel, validateFilesNamesRegex []*regexp.Regexp) (ApplicationConfig, error) {
	cfg := ApplicationConfig{
		channels: channels,
	}
	channelsLen := len(cfg.channels)
	channelsUniqNames := make(map[string]struct{}, channelsLen)
	for i, channel := range cfg.channels {
		if _, ok := channelsUniqNames[channel.name]; ok {
			return ApplicationConfig{}, fmt.Errorf("channel name \"%s\" is not uniq", channel.name)
		}
		channelsUniqNames[channel.name] = struct{}{}
		cfg.channels[i].weight = channelsLen - i
	}
	curVersion, err := newVersionCurrent(cfg, version)
	if err != nil {
		return ApplicationConfig{}, err
	}
	cfg.currentVersion = curVersion
	if len(validateFilesNamesRegex) == 0 {
		validateFilesNamesRegex = []*regexp.Regexp{defaultValidFileNameRegex}
	}
	cfg.ValidateFilesNamesRegexes = validateFilesNamesRegex
	return cfg, nil
}

func (ac *ApplicationConfig) getReleaseChannel() *Channel {
	for _, channel := range ac.channels {
		if channel.isReleaseChan {
			return &channel
		}
	}
	return nil
}

type UpdateConfig struct {
	ApplicationConfig ApplicationConfig
	Sources           []UpdateSource // source oder is source PRIORITY
}
