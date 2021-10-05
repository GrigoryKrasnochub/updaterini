package updaterini

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"
)

var ErrorResponseCodeIsNotOK = errors.New("error. response code is not OK")

const (
	SourceLabelGitRepo = "SourceGitRepo"
	SourceLabelServer  = "SourceServer"
)

type UpdateSource interface {
	SourceLabel() string
	getSourceVersions(cfg ApplicationConfig) ([]Version, SourceStatus)
}

type UpdateSourceGitRepo struct {
	UserName                 string
	RepoName                 string
	SkipGitReleaseDraftCheck bool   // on true releases marked as draft won't be skipped
	PersonalAccessToken      string // ONLY FOR DEBUG PURPOSE
}

func (sGit *UpdateSourceGitRepo) SourceLabel() string {
	return SourceLabelGitRepo
}

func (sGit *UpdateSourceGitRepo) getSourceUrl() string {
	link := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", sGit.UserName, sGit.RepoName)
	return link
}

func (sGit *UpdateSourceGitRepo) getLoadFileUrl(fileId int) string {
	link := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/assets/%d", sGit.UserName, sGit.RepoName, fileId)
	return link
}

func (sGit *UpdateSourceGitRepo) getSourceVersions(cfg ApplicationConfig) (resultVersions []Version, srcStatus SourceStatus) {
	srcStatus.Source = sGit
	resp, err := doGetRequest(sGit.getSourceUrl(), cfg, nil, nil)
	if err != nil {
		srcStatus.appendError(err, true)
		return nil, srcStatus
	}
	defer func() {
		tmpErr := resp.Body.Close()
		if tmpErr != nil {
			srcStatus.appendError(tmpErr, false)
		}
	}()
	var data []gitData
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		srcStatus.appendError(err, true)
		return nil, srcStatus
	}
	for _, gData := range data {
		gVersion, err := newVersionGit(cfg, gData, *sGit)
		if err != nil {
			if cfg.ShowPrepareVersionErr {
				srcStatus.appendError(err, false)
			}
			continue
		}
		resultVersions = append(resultVersions, &gVersion)
	}
	return resultVersions, srcStatus
}

func (sGit *UpdateSourceGitRepo) loadSourceFile(cfg ApplicationConfig, fileId int) (io.ReadCloser, error) {
	customHeaders := make(map[string]string, 2)
	customHeaders["Accept"] = "application/octet-stream"
	if sGit.PersonalAccessToken != "" {
		customHeaders["Authorization"] = fmt.Sprintf("token %s", sGit.PersonalAccessToken)
	}
	resp, err := doGetRequest(sGit.getLoadFileUrl(fileId), cfg, customHeaders,
		map[int]interface{}{200: struct{}{}, 302: struct{}{}})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

type UpdateSourceServer struct {
	UpdatesMapURL string
}

func (sServ *UpdateSourceServer) SourceLabel() string {
	return SourceLabelServer
}

func (sServ *UpdateSourceServer) getSourceVersions(cfg ApplicationConfig) (resultVersions []Version, srcStatus SourceStatus) {
	srcStatus.Source = sServ
	resp, err := doGetRequest(sServ.UpdatesMapURL, cfg, nil, nil)
	if err != nil {
		srcStatus.appendError(err, true)
		return nil, srcStatus
	}
	defer func() {
		tmpErr := resp.Body.Close()
		if tmpErr != nil {
			srcStatus.appendError(tmpErr, false)
		}
	}()
	var sData []ServData
	err = json.NewDecoder(resp.Body).Decode(&sData)
	if err != nil {
		srcStatus.appendError(err, true)
		return nil, srcStatus
	}
	for _, data := range sData {
		version, err := newVersionServ(cfg, data, *sServ)
		if err != nil {
			if cfg.ShowPrepareVersionErr {
				srcStatus.appendError(err, false)
			}
			continue
		}
		resultVersions = append(resultVersions, &version)
	}
	return resultVersions, srcStatus
}

func (sServ *UpdateSourceServer) loadSourceFile(cfg ApplicationConfig, serverFolderUrl, filename string) (io.ReadCloser, error) {
	resp, err := doGetRequest(serverFolderUrl+filename, cfg, nil, map[int]interface{}{200: struct{}{}})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

var reqHTTP = &http.Client{
	Timeout: 5 * time.Minute,
	Transport: &http.Transport{
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	},
}

func doGetRequest(url string, appConfig ApplicationConfig, customHeaders map[string]string, okCodes map[int]interface{}) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf(`updaterini %s (%s %s-%s)`, appConfig.currentVersion.version.String(), runtime.Version(), runtime.GOOS, runtime.GOARCH))
	for key, customHeader := range customHeaders {
		req.Header.Set(key, customHeader)
	}
	resp, err := reqHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if _, ok := okCodes[resp.StatusCode]; (len(okCodes) != 0 || resp.StatusCode != 200) && !ok {
		_ = resp.Body.Close()
		return nil, ErrorResponseCodeIsNotOK
	}
	return resp, nil
}
