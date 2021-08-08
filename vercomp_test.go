package updaterini

import (
	"testing"

	"github.com/blang/semver/v4"
)

type versionTest struct {
	version      string
	isMaxVersion bool
}

func testLatestVersionSearch(curVersion versionTest, channels []Channel, testVers []versionTest, t *testing.T) {
	maxVersionsCounter := 0
	if curVersion.isMaxVersion {
		maxVersionsCounter += 1
	}
	for _, vTest := range testVers {
		if vTest.isMaxVersion {
			maxVersionsCounter += 1
		}
	}
	if maxVersionsCounter != 1 {
		t.Errorf("bad tests. more than one version is max")
	}

	cfg, err := NewApplicationConfig(curVersion.version, channels, nil)
	if err != nil {
		t.Errorf("creating new version err: %s", err)
	}
	versions := make([]Version, 0)
	for _, testVer := range testVers {
		ver, err := newVersionGit(cfg, gitData{
			Version: testVer.version,
			Assets: []struct {
				Size     int
				Id       int
				Filename string `json:"name"`
				Url      string `json:"browser_download_url"`
			}{
				{Size: 1, Id: 1, Filename: "v1.0.1_linux_amd64", Url: ""},
				{Size: 1, Id: 2, Filename: "v1.0.1_windows_amd64", Url: ""},
			},
		}, UpdateSourceGitRepo{
			UserName: "",
			RepoName: "",
		})
		if err != nil {
			t.Errorf("create new version err: %s", err)
			continue
		}
		versions = append(versions, &ver)
	}
	ver := getLatestVersion(cfg, versions)
	if ver == nil && curVersion.isMaxVersion {
		return
	}
	for index, testVer := range testVers {
		if testVer.isMaxVersion && testVer.version != ver.VersionTag() {
			t.Errorf("compare test version err: max version is not max")
		}
		if !testVer.isMaxVersion && testVer.version == ver.VersionTag() {
			t.Errorf("compare test version err: not max version is max. max index:%d. max version tag:%s", index, testVer.version)
		}
	}
}

func TestSemverLibVersionComparison(t *testing.T) {
	strMaxVersion := "1.0.1-dev.12.2"

	strVersions := []string{
		"1.0.1-dev.1",
		"1.0.1-dev.1.3",
		"1.0.1-dev.9.3",
		strMaxVersion,
		"1.0.1-beta.1",
		"1.0.1-alpha.1",
	}

	versions := make([]semver.Version, len(strVersions))
	var err error
	for i, strVersion := range strVersions {
		versions[i], err = semver.Parse(strVersion)
		if err != nil {
			t.Errorf("version parse err: %s", err)
		}
	}
	var maxVersion semver.Version
	for _, version := range versions {
		res := version.Compare(maxVersion)
		if res == 1 {
			maxVersion = version
		}
	}
	if maxVersion.String() != strMaxVersion {
		t.Errorf("version comparison err. should be:%s . now:%s", strMaxVersion, maxVersion.String())
	}
}

func TestVersionComparisonReleaseOnly(t *testing.T) {
	versionTests := []versionTest{
		{
			version:      "1.0.3",
			isMaxVersion: false,
		},
		{
			version:      "1.2.0",
			isMaxVersion: false,
		},
		{
			version:      "3.2.1",
			isMaxVersion: false,
		},
		{
			version:      "4.2.1",
			isMaxVersion: false,
		},
		{
			version:      "3.1.1",
			isMaxVersion: false,
		},
		{
			version:      "4.2.2+123",
			isMaxVersion: true,
		},
		{
			version:      "4.2.2+223",
			isMaxVersion: false,
		},
	}
	channels := []Channel{
		NewReleaseChannel(true),
	}
	testLatestVersionSearch(versionTest{version: "1.0.0", isMaxVersion: false}, channels, versionTests, t)
}

func TestVersionComparisonRelDev1(t *testing.T) {
	versionTests := []versionTest{
		{
			version:      "1.0.1",
			isMaxVersion: true,
		},
	}
	channels := []Channel{
		NewReleaseChannel(true),
		NewChannel("dev", true),
	}
	testLatestVersionSearch(versionTest{version: "1.0.1-dev.1", isMaxVersion: false}, channels, versionTests, t)
}

func TestVersionComparisonRelDev2(t *testing.T) {
	versionTests := []versionTest{
		{
			version:      "1.0.1-dev.0.1",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1-dev.1.4",
			isMaxVersion: false,
		},
		{
			version:      "1.0.0",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1-dev.0.9",
			isMaxVersion: false,
		},
	}
	channels := []Channel{
		NewReleaseChannel(true),
		NewChannel("dev", true),
	}
	testLatestVersionSearch(versionTest{version: "1.0.1-dev.1.5", isMaxVersion: true}, channels, versionTests, t)
}

func TestVersionComparisonRelDev3(t *testing.T) {
	versionTests := []versionTest{
		{
			version:      "1.0.1-dev",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1-dev.0.1",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1-dev.1.5",
			isMaxVersion: true,
		},
		{
			version:      "1.0.0",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1-dev.0.9",
			isMaxVersion: false,
		},
	}
	channels := []Channel{
		NewReleaseChannel(true),
		NewChannel("dev", true),
	}
	testLatestVersionSearch(versionTest{version: "1.0.1-dev.1.4", isMaxVersion: false}, channels, versionTests, t)
}

func TestVersionComparisonMultiChan1(t *testing.T) {
	versionTests := []versionTest{
		{
			version:      "1.0.1-alpha.1.4",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1-beta.1.4",
			isMaxVersion: true,
		},
		{
			version:      "1.0.1-dev.1.4",
			isMaxVersion: false,
		},
	}
	channels := []Channel{
		NewReleaseChannel(true),
		NewChannel("beta", true),
		NewChannel("alpha", true),
		NewChannel("dev", true),
	}
	testLatestVersionSearch(versionTest{version: "1.0.0", isMaxVersion: false}, channels, versionTests, t)
}

func TestVersionComparisonMultiChan2(t *testing.T) {
	versionTests := []versionTest{
		{
			version:      "1.0.1-alpha.1.4",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1-alpha.1.5",
			isMaxVersion: true,
		},
		{
			version:      "1.0.1-beta.1.4",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1-dev.1.4",
			isMaxVersion: false,
		},
	}
	channels := []Channel{
		NewReleaseChannel(true),
		NewChannel("beta", true),
		NewChannel("alpha", true),
		NewChannel("dev", true),
	}
	testLatestVersionSearch(versionTest{version: "1.0.0", isMaxVersion: false}, channels, versionTests, t)
}

func TestVersionComparisonMultiChan3(t *testing.T) {
	versionTests := []versionTest{
		{
			version:      "1.0.1-alpha.1.4",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1",
			isMaxVersion: true,
		},
		{
			version:      "1.0.1-beta.1.4",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1-dev.1.4",
			isMaxVersion: false,
		},
	}
	channels := []Channel{
		NewReleaseChannel(true),
		NewChannel("beta", true),
		NewChannel("alpha", true),
		NewChannel("dev", true),
	}
	testLatestVersionSearch(versionTest{version: "1.0.0", isMaxVersion: false}, channels, versionTests, t)
}

func TestVersionComparisonMultiChan4(t *testing.T) {
	versionTests := []versionTest{
		{
			version:      "1.0.1-alpha.1.4",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1",
			isMaxVersion: true,
		},
		{
			version:      "1.0.1-beta.1.4",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1-dev.1.4",
			isMaxVersion: false,
		},
	}
	channels := []Channel{
		NewChannel("dev", true),
		NewChannel("beta", true),
		NewReleaseChannel(true),
		NewChannel("alpha", true),
	}
	testLatestVersionSearch(versionTest{version: "1.0.0", isMaxVersion: false}, channels, versionTests, t)
}

func TestVersionComparisonMultiChan5(t *testing.T) {
	versionTests := []versionTest{
		{
			version:      "1.0.1-alpha.1.4",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1-alpha.1.5.0.1",
			isMaxVersion: true,
		},
		{
			version:      "1.0.1-beta.1.5",
			isMaxVersion: false,
		},
		{
			version:      "1.0.1-dev.1.4",
			isMaxVersion: false,
		},
	}
	channels := []Channel{
		NewChannel("dev", true),
		NewChannel("beta", true),
		NewReleaseChannel(true),
		NewChannel("alpha", true),
	}
	testLatestVersionSearch(versionTest{version: "1.0.0", isMaxVersion: false}, channels, versionTests, t)
}
