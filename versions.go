package updaterini

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/blang/semver/v4"
)

const (
	errorVersionParseErrNumericPreRelease = "version parse err: numeric pre-release branches are unsupported"
	errorVersionParseErrNoChannel         = "version parse err: can't find channel"
	errorVersionInvalid                   = "version is invalid (no files/invalid files names)"

	errorAssetNotFoundByFilename = "asset not found by filename"
)

func isVersionFilenameCorrect(filename string, filenameRegex []*regexp.Regexp) bool {
	for _, regex := range filenameRegex {
		if regex.MatchString(filename) {
			return true
		}
	}
	return false
}

type Version interface {
	getVersion() semver.Version
	getChannel() Channel
	getAssetsFilenames() []string
	getAssetContentByFilename(cfg ApplicationConfig, filename string) (io.ReadCloser, error)
	VersionName() string
	VersionTag() string
	VersionDescription() string
}

func getLatestVersion(cfg ApplicationConfig, versions []Version) Version {
	maxVersionIndex := -1
	maxVersion := prepareVersionForComparison(cfg.currentVersion.version)
	maxVersionChanWeight := cfg.currentVersion.channel.weight
	for i := 0; i < len(versions); i++ {
		verChan := versions[i].getChannel()
		if !verChan.useForUpdate {
			continue
		}
		verChanWeight := versions[i].getChannel().weight
		prepVersion := prepareVersionForComparison(versions[i].getVersion())
		if compareResult := prepVersion.Compare(maxVersion); compareResult == 1 || (maxVersionChanWeight < verChanWeight && compareResult == 0) {
			maxVersionIndex = i
			maxVersion = prepVersion
			maxVersionChanWeight = verChanWeight
		}
	}
	if maxVersionIndex == -1 {
		return nil
	}
	return versions[maxVersionIndex]
}

func prepareVersionForComparison(version semver.Version) semver.Version {
	if len(version.Pre) > 0 {
		version.Pre = version.Pre[1:]
		// for preventing change Pre version to release
		version.Pre = append(version.Pre, semver.PRVersion{VersionNum: 0, IsNum: true})
	}
	return version
}

func prepareVersionString(version string) string {
	return strings.TrimLeft(strings.TrimSpace(version), "v")
}

func parseVersion(cfg ApplicationConfig, version string) (semver.Version, Channel, error) {
	version = prepareVersionString(version)
	parsedVersion, err := semver.Parse(version)
	if err != nil {
		return parsedVersion, Channel{}, fmt.Errorf("%s: %s", version, err)
	}
	if len(parsedVersion.Pre) == 0 {
		rChan := cfg.getReleaseChannel()
		if rChan != nil {
			return parsedVersion, *rChan, nil
		}
		return parsedVersion, Channel{}, fmt.Errorf("%s: %s", version, errorVersionParseErrNoChannel)
	}
	if parsedVersion.Pre[0].IsNum {
		return parsedVersion, Channel{}, fmt.Errorf("%s: %s", version, errorVersionParseErrNumericPreRelease)
	}
	for _, channel := range cfg.channels {
		if channel.name == parsedVersion.Pre[0].VersionStr {
			return parsedVersion, channel, nil
		}
	}
	return parsedVersion, Channel{}, fmt.Errorf("%s: %s", version, errorVersionParseErrNoChannel)
}

type gitData struct {
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	Name        string    `json:"name"`
	ReleaseDate time.Time `json:"published_at"`
	Description string    `json:"body"`
	Version     string    `json:"tag_name"`
	Assets      []struct {
		Size     int
		Id       int
		Filename string `json:"name"`
		Url      string `json:"browser_download_url"`
	}
}

type versionGit struct {
	data    gitData
	channel Channel
	source  UpdateSourceGitRepo
	version semver.Version
}

func newVersionGit(cfg ApplicationConfig, data gitData, src UpdateSourceGitRepo) (versionGit, error) {
	vG := versionGit{
		data: data,
	}
	if !vG.isValid(cfg.ValidateFilesNamesRegexes) {
		return versionGit{}, fmt.Errorf("%s: %s", data.Version, errorVersionInvalid)
	}
	vG.cleanUnusedAssets(cfg.ValidateFilesNamesRegexes)
	version, channel, err := parseVersion(cfg, data.Version)
	if err != nil {
		return versionGit{}, err
	}
	vG.version = version
	vG.channel = channel
	vG.source = src
	return vG, nil
}

func (vG *versionGit) VersionName() string {
	return vG.data.Name
}

func (vG *versionGit) VersionTag() string {
	return vG.data.Version
}

func (vG *versionGit) VersionDescription() string {
	return vG.data.Description
}

func (vG *versionGit) isValid(filenameRegex []*regexp.Regexp) bool {
	for _, gitAsset := range vG.data.Assets {
		if isVersionFilenameCorrect(gitAsset.Filename, filenameRegex) {
			return true
		}
	}
	return false
}

func (vG *versionGit) cleanUnusedAssets(filenameRegex []*regexp.Regexp) {
	assetsCounter := 0
	for _, asset := range vG.data.Assets {
		if isVersionFilenameCorrect(asset.Filename, filenameRegex) {
			vG.data.Assets[assetsCounter] = asset
			assetsCounter++
		}
	}
	vG.data.Assets = vG.data.Assets[:assetsCounter]
}

func (vG *versionGit) getVersion() semver.Version {
	return vG.version
}

func (vG *versionGit) getChannel() Channel {
	return vG.channel
}

func (vG *versionGit) getAssetsFilenames() []string {
	result := make([]string, len(vG.data.Assets), len(vG.data.Assets))
	for i, asset := range vG.data.Assets {
		result[i] = asset.Filename
	}
	return result
}

func (vG *versionGit) getAssetContentByFilename(cfg ApplicationConfig, filename string) (io.ReadCloser, error) {
	for _, asset := range vG.data.Assets {
		if asset.Filename != filename {
			continue
		}
		return vG.source.loadSourceFile(cfg, asset.Id)
	}
	return nil, errors.New(errorAssetNotFoundByFilename)
}

type ServData struct {
	VersionFolderUrl string `json:"folder_url"` // version folder url
	Name             string // release summary
	Description      string // release description
	Version          string // version tag
	Assets           []struct { // version files
		Filename string // version files filenames, filenames adds to VersionFolderUrl
	}
}

type versionServ struct {
	data    ServData
	channel Channel
	source  UpdateSourceServer
	version semver.Version
}

func newVersionServ(cfg ApplicationConfig, data ServData, src UpdateSourceServer) (versionServ, error) {
	vS := versionServ{
		data: data,
	}
	if !vS.isValid(cfg.ValidateFilesNamesRegexes) {
		return versionServ{}, fmt.Errorf("%s: %s", data.Version, errorVersionInvalid)
	}
	vS.cleanUnusedAssets(cfg.ValidateFilesNamesRegexes)

	version, channel, err := parseVersion(cfg, data.Version)
	if err != nil {
		return versionServ{}, err
	}
	vS.version = version
	vS.channel = channel
	vS.source = src
	return vS, nil
}

func (vS *versionServ) VersionName() string {
	return vS.data.Name
}

func (vS *versionServ) VersionTag() string {
	return vS.data.Version
}

func (vS *versionServ) VersionDescription() string {
	return vS.data.Description
}

func (vS *versionServ) isValid(filenameRegex []*regexp.Regexp) bool {
	for _, asset := range vS.data.Assets {
		if isVersionFilenameCorrect(asset.Filename, filenameRegex) {
			return true
		}
	}
	return false
}

func (vS *versionServ) cleanUnusedAssets(filenameRegex []*regexp.Regexp) {
	assetsCounter := 0
	for _, asset := range vS.data.Assets {
		if isVersionFilenameCorrect(asset.Filename, filenameRegex) {
			vS.data.Assets[assetsCounter] = asset
			assetsCounter++
		}
	}
	vS.data.Assets = vS.data.Assets[:assetsCounter]
}

func (vS *versionServ) getVersion() semver.Version {
	return vS.version
}

func (vS *versionServ) getChannel() Channel {
	return vS.channel
}

func (vS *versionServ) getAssetsFilenames() []string {
	result := make([]string, len(vS.data.Assets), len(vS.data.Assets))
	for i, asset := range vS.data.Assets {
		result[i] = asset.Filename
	}
	return result
}

func (vS *versionServ) getAssetContentByFilename(cfg ApplicationConfig, filename string) (io.ReadCloser, error) {
	for _, asset := range vS.data.Assets {
		if asset.Filename != filename {
			continue
		}
		return vS.source.loadSourceFile(cfg, vS.data.VersionFolderUrl, asset.Filename)
	}
	return nil, errors.New(errorAssetNotFoundByFilename)
}
