package updaterini

import (
	"errors"
	"strings"
	"time"

	"github.com/blang/semver/v4"
)

var (
	ErrorUndefinedChannel = errors.New("channel is undefined")
	ErrorIncorrectVersion = errors.New("parse version variant err")
)

type Version interface {
	getVersion() semver.Version
	getChannel() Channel
}

func getLatestVersion(cfg applicationConfig, versions []Version) (*Version, *int) {
	maxVersionIndex := -1
	maxVersion := prepareVersionForComparison(cfg.currentVersion.getChannel(), cfg.currentVersion.getVersion())
	for i := 0; i < len(versions); i++ {
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
		version.Pre[0].VersionNum = uint64(channel.Weight)
		version.Pre[0].VersionStr = ""
		version.Pre[0].IsNum = true
	}
	return version
}

func prepareVersionString(version string) string {
	return strings.TrimLeft(strings.TrimSpace(version), "v")
}

func parseVersion(appConf applicationConfig, version string) (*semver.Version, *Channel, error) {
	version = prepareVersionString(version)
	parsedVersion, err := semver.Parse(version)
	if err != nil {
		return nil, nil, err
	}
	if len(parsedVersion.Pre) == 0 && appConf.isReleaseChanelAvailable() {
		rChan := GetReleaseChanel()
		return &parsedVersion, &rChan, nil
	}
	if parsedVersion.Pre[0].IsNum {
		return &parsedVersion, nil, errors.New("version parse err numeric pre-release branches are unsupported") //TODO NOTICE NOT ERR
	}
	for _, channel := range appConf.channels {
		if channel.Name == parsedVersion.Pre[0].VersionStr {
			return &parsedVersion, &channel, nil
		}
	}
	// when compare change pre[0] to priority val, ONLY POSITIVE
	return &parsedVersion, nil, nil
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

func newVersionGit(cfg applicationConfig, data gitData) *versionGit {
	vG := &versionGit{
		data: data,
	}
	if !vG.isValid(cfg) {
		return nil
	}
	version, channel, err := parseVersion(cfg, data.version)
	if channel == nil || err != nil {
		return nil
	}
	vG.version = *version
	vG.channel = *channel
	return vG
}

func (vG *versionGit) isValid(cfg applicationConfig) bool {
	release := true
	if vG.data.prerelease {
		release = !cfg.isReleaseChanelOnlyMod()
	}
	return !vG.data.draft && release
}

func (vG *versionGit) GetVersion() string {
	return vG.version.String()
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

func newVersionCurrent(appConf applicationConfig, version string) (*versionCurrent, error) {
	pVersion, channel, err := parseVersion(appConf, version)
	if err != nil {
		return nil, err
	}
	curVer := &versionCurrent{
		version: *pVersion,
	}
	if channel == nil {
		curVer.channel = Channel{Weight: 0}
		return curVer, ErrorUndefinedChannel // TODO notice not err
	}
	curVer.channel = *channel
	return curVer, nil
}

func (vC *versionCurrent) GetVersion() string {
	return vC.version.String()
}

func (vC *versionCurrent) getVersion() semver.Version {
	return vC.version
}

func (vC *versionCurrent) getChannel() Channel {
	return vC.channel
}
