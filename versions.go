package updaterini

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/blang/semver/v4"
)

var (
	ErrorUndefinedChannel                 = errors.New("channel is undefined")
	ErrorVersionParseErrNumericPreRelease = errors.New("version parse err: numeric pre-release branches are unsupported")
	ErrorVersionParseErrNoChannel         = errors.New("version parse err: can't find channel")
	ErrorVersionInvalid                   = errors.New("version is invalid")

	ValidFileNameRegex = regexp.MustCompile(fmt.Sprintf(".*%s_%s.*", runtime.GOOS, runtime.GOARCH))
)

func isVersionFilenameCorrect(filename string) bool {
	return ValidFileNameRegex.MatchString(filename)
}

type version interface {
	getVersion() semver.Version
	getChannel() Channel
	getAssetsFilesContent(cfg applicationConfig, processFileContent func(reader io.Reader, filename string) error) error
}

func getLatestVersion(cfg applicationConfig, versions []version) (*version, *int) {
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
	Size     int
	Id       int
	Filename string `json:"name"`
	Url      string `json:"browser_download_url"`
}

type gitData struct {
	Prerelease  bool        `json:"prerelease"`
	Draft       bool        `json:"draft"`
	Name        string      `json:"name"`
	ReleaseDate time.Time   `json:"published_at"`
	Description string      `json:"body"`
	Version     string      `json:"tag_name"`
	Assets      []gitAssets `json:"assets"`
}

type versionGit struct {
	data    gitData
	channel Channel
	source  UpdateSourceGitRepo
	version semver.Version
}

func newVersionGit(cfg applicationConfig, data gitData, src UpdateSourceGitRepo) (*versionGit, error) {
	vG := &versionGit{
		data: data,
	}
	if !vG.isValid(cfg) {
		return nil, ErrorVersionInvalid
	}
	vG.cleanUnusedAssets()
	version, channel, err := parseVersion(cfg, data.Version)
	if err != nil {
		return nil, err
	}
	vG.version = *version
	vG.channel = *channel
	vG.source = src
	return vG, nil
}

func (vG *versionGit) isValid(cfg applicationConfig) bool {
	release := true
	if vG.data.Prerelease {
		release = !cfg.isReleaseChannelOnlyMod()
	}
	files := false
	for _, gitAsset := range vG.data.Assets {
		if isVersionFilenameCorrect(gitAsset.Filename) {
			files = true
		}
	}
	return !vG.data.Draft && release && files
}

func (vG *versionGit) cleanUnusedAssets() {
	var cleanAssets []gitAssets
	for _, gitAsset := range vG.data.Assets {
		if isVersionFilenameCorrect(gitAsset.Filename) {
			cleanAssets = append(cleanAssets, gitAsset)
		}
	}
	vG.data.Assets = cleanAssets
}

func (vG *versionGit) getVersion() semver.Version {
	return vG.version
}

func (vG *versionGit) getChannel() Channel {
	return vG.channel
}

func (vG *versionGit) getAssetsFilesContent(cfg applicationConfig, processFileContent func(reader io.Reader, filename string) error) error {
	for _, asset := range vG.data.Assets {
		reader, err := vG.source.loadSourceFile(cfg, asset.Id)
		if err != nil {
			return err
		}
		err = processFileContent(reader, asset.Filename)
		if err != nil {
			return err
		}
		err = reader.Close()
		if err != nil {
			return err
		}
	}
	return nil
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

func (vC *versionCurrent) getAssetsFilesContent(applicationConfig, func(reader io.Reader, filename string) error) error {
	return nil
}
