package updaterini

import (
	"errors"
	"strings"
	"time"

	"github.com/blang/semver/v4"
)

var (
	ErrorUndefinedChannel                 = errors.New("channel is undefined")
	ErrorVersionParseErrNumericPreRelease = errors.New("version parse err: numeric pre-release branches are unsupported")
	ErrorVersionParseErrNoChannel         = errors.New("version parse err: can't find channel")
	ErrorVersionInvalid                   = errors.New("version is invalid")
)

type Version interface {
	getVersion() semver.Version
	getChannel() Channel
}

func getLatestVersion(cfg applicationConfig, versions []Version) (*Version, *int) {
	maxVersionIndex := -1
	maxVersion := prepareVersionForComparison(cfg.currentVersion.getChannel(), cfg.currentVersion.getVersion())
	for i := 0; i < len(versions); i++ {
		if !versions[i].getChannel().useForUpdate {
			continue
		}
		prepVersion := prepareVersionForComparison(versions[i].getChannel(), versions[i].getVersion())
		if prepVersion.Compare(maxVersion) == 1 {
			maxVersionIndex = i
			maxVersion = prepVersion
		}
	}
	if maxVersionIndex == -1 {
		return nil, nil
	}
	return &versions[maxVersionIndex], &maxVersionIndex
}

func prepareVersionForComparison(channel Channel, version semver.Version) semver.Version {
	if len(version.Pre) > 0 {
		version.Pre[0].VersionNum = uint64(channel.weight)
		version.Pre[0].VersionStr = ""
		version.Pre[0].IsNum = true
	}
	return version
}

func prepareVersionString(version string) string {
	return strings.TrimLeft(strings.TrimSpace(version), "v")
}

func parseVersion(cfg applicationConfig, version string) (*semver.Version, *Channel, error) {
	version = prepareVersionString(version)
	parsedVersion, err := semver.Parse(version)
	if err != nil {
		return nil, nil, err
	}
	if len(parsedVersion.Pre) == 0 {
		rChan := cfg.getReleaseChannel()
		if rChan != nil {
			return &parsedVersion, rChan, nil
		}
		return &parsedVersion, nil, ErrorVersionParseErrNoChannel
	}
	if parsedVersion.Pre[0].IsNum {
		return &parsedVersion, nil, ErrorVersionParseErrNumericPreRelease
	}
	for _, channel := range cfg.channels {
		if channel.Name == parsedVersion.Pre[0].VersionStr {
			return &parsedVersion, &channel, nil
		}
	}
	return &parsedVersion, nil, ErrorVersionParseErrNoChannel
}

type gitAssets struct {
	size int
	id   int
	url  string `json:"browser_download_url"`
}

type gitData struct {
	prerelease  bool      `json:"prerelease"`
	draft       bool      `json:"draft"`
	name        string    `json:"name"`
	releaseDate time.Time `json:"published_at"`
	description string    `json:"body"`
	version     string    `json:"tag_name"`
	assets      []gitAssets
}

type versionGit struct {
	data    gitData
	channel Channel
	version semver.Version
}

func newVersionGit(cfg applicationConfig, data gitData) (*versionGit, error) {
	vG := &versionGit{
		data: data,
	}
	if !vG.isValid(cfg) {
		return nil, ErrorVersionInvalid
	}
	version, channel, err := parseVersion(cfg, data.version)
	if err != nil {
		return nil, err
	}
	vG.version = *version
	vG.channel = *channel
	return vG, nil
}

func (vG *versionGit) isValid(cfg applicationConfig) bool {
	release := true
	if vG.data.prerelease {
		release = !cfg.isReleaseChannelOnlyMod()
	}
	return !vG.data.draft && release
}

func (vG *versionGit) getVersion() semver.Version {
	return vG.version
}

func (vG *versionGit) getChannel() Channel {
	return vG.channel
}

type versionCurrent struct {
	channel Channel
	version semver.Version
}

func newVersionCurrent(cfg applicationConfig, version string) (*versionCurrent, error) {
	pVersion, channel, err := parseVersion(cfg, version)
	if err != nil {
		return nil, err
	}
	curVer := &versionCurrent{
		version: *pVersion,
	}
	if channel == nil {
		return nil, ErrorUndefinedChannel
	}
	curVer.channel = *channel
	return curVer, nil
}

func (vC *versionCurrent) getVersion() semver.Version {
	return vC.version
}

func (vC *versionCurrent) getChannel() Channel {
	return vC.channel
}
