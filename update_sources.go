package updaterini

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"runtime"
	"time"
)

type UpdateSource interface {
	getSourceVersions(appConfig ApplicationConfig)
}

type UpdateSourceUrl interface {
	getSourceUrl() string
}

type UpdateSourceGitRepo struct {
	UserName string
	RepoName string
}

func (sGit *UpdateSourceGitRepo) getSourceUrl() string {
	// For all release remove "latest" TODO check with Channels
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", sGit.UserName, sGit.RepoName)
}

func (sGit *UpdateSourceGitRepo) getSourceVersions(appConfig ApplicationConfig) {
	_, _ = doSourceRequest(sGit, appConfig)
}

type UpdateSourceHTTPServ struct {
	URL string // description file url
}

func (sHTTP *UpdateSourceHTTPServ) getSourceUrl() string {
	return sHTTP.URL
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

func doSourceRequest(usu UpdateSourceUrl, appConfig ApplicationConfig) (*http.Response, error) {
	req, err := http.NewRequest("GET", usu.getSourceUrl(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf(`updaterini %s (%s %s-%s)`, appConfig.CurrentVersion, runtime.Version(), runtime.GOOS, runtime.GOARCH))
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
