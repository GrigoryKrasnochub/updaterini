package updaterini

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"
)

type UpdateSource interface {
	getSourceVersions(cfg applicationConfig) ([]Version, error)
}

type UpdateSourceUrl interface {
	getSourceUrl(cfg applicationConfig) string
}

type UpdateSourceGitRepo struct {
	UserName string
	RepoName string
}

func (sGit *UpdateSourceGitRepo) getSourceUrl(cfg applicationConfig) string {
	link := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", sGit.UserName, sGit.RepoName)
	if cfg.isReleaseChannelOnlyMod() {
		link += "/latest"
	}
	return link
}

func (sGit *UpdateSourceGitRepo) getSourceVersions(cfg applicationConfig) ([]Version, error) {
	resp, err := doSourceRequest(sGit, cfg)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = resp.Body.Close()
	}()
	jD := json.NewDecoder(resp.Body)
	var data []gitData
	if cfg.isReleaseChannelOnlyMod() {
		var tmpData gitData
		err = jD.Decode(&tmpData)
		data = append(data, tmpData)
	} else {
		err = jD.Decode(&data)
	}
	if err != nil {
		return nil, err
	}
	var resultVersions []Version
	for _, gData := range data {
		gVersion, err := newVersionGit(cfg, gData)
		if cfg.ShowPrepareVersionErr && err != nil {
			return nil, err
		}
		resultVersions = append(resultVersions, gVersion)
	}
	return resultVersions, nil
}

const readTimeout = 30 * time.Minute

var insecureHTTP = &http.Client{
	Timeout: readTimeout,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

func doSourceRequest(usu UpdateSourceUrl, appConfig applicationConfig) (*http.Response, error) {
	req, err := http.NewRequest("GET", usu.getSourceUrl(appConfig), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf(`updaterini %s (%s %s-%s)`, appConfig.currentVersion.getVersion().String(), runtime.Version(), runtime.GOOS, runtime.GOARCH))
	resp, err := insecureHTTP.Do(req)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	if resp.StatusCode != 200 {
		fmt.Println("HTTP error:", resp.Status)
		return nil, err
	}
	return resp, nil
}
