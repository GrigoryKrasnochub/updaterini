package updaterini

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"
)

type UpdateSource interface {
	getSourceVersions(cfg applicationConfig) ([]Version, error)
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

func (sGit *UpdateSourceGitRepo) getLoadFileUrl(fileId int) string {
	link := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/assets/%d", sGit.UserName, sGit.RepoName, fileId)
	return link
}

func (sGit *UpdateSourceGitRepo) getSourceVersions(cfg applicationConfig) ([]Version, error) {
	resp, err := doGetRequest(sGit.getSourceUrl(cfg), cfg, nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resp != nil {
			err = resp.Body.Close()
		}
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
		gVersion, err := newVersionGit(cfg, gData, *sGit)
		if err != nil {
			if cfg.ShowPrepareVersionErr {
				return nil, err
			}
			continue
		}
		resultVersions = append(resultVersions, gVersion)
	}
	return resultVersions, nil
}

func (sGit *UpdateSourceGitRepo) loadSourceFile(cfg applicationConfig, fileId int) (io.ReadCloser, error) {
	resp, err := doGetRequest(sGit.getLoadFileUrl(fileId), cfg, map[string]string{"Accept": "application/octet-stream"},
		map[int]interface{}{200: struct{}{}, 302: struct{}{}})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
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

func doGetRequest(url string, appConfig applicationConfig, customHeaders map[string]string, okCodes map[int]interface{}) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf(`updaterini %s (%s %s-%s)`, appConfig.currentVersion.getVersion().String(), runtime.Version(), runtime.GOOS, runtime.GOARCH))
	for key, customHeader := range customHeaders {
		req.Header.Set(key, customHeader)
	}
	resp, err := insecureHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if _, ok := okCodes[resp.StatusCode]; !(len(okCodes) == 0 && resp.StatusCode == 200) && !ok {
		return nil, err
	}
	return resp, nil
}
