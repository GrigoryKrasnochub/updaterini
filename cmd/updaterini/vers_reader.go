package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/GrigoryKrasnochub/updaterini"
	"github.com/blang/semver/v4"
)

type verReader struct {
	versionsDir              string
	descriptionFilename      string
	descriptionNameSeparator string
}

type servExtData struct {
	updaterini.ServData
	version semver.Version
}

func (vr verReader) readVersionsDir() ([]servExtData, error) {
	versions, err := ioutil.ReadDir(vr.versionsDir)
	versionsSDData := make([]servExtData, 0)
	if err != nil {
		return nil, fmt.Errorf("read versions dir error: %v", err)
	}
	for _, version := range versions {
		if !version.IsDir() {
			continue
		}
		pVer, err := updaterini.ParseVersion(version.Name())
		if err != nil {
			return nil, fmt.Errorf("parse version folder name error (incorrect version): %v", err)
		}
		sData, err := vr.readVersionDir(filepath.Join(vr.versionsDir, version.Name()))
		if err != nil {
			return nil, err
		}
		sData.version = pVer
		versionsSDData = append(versionsSDData, sData)
	}
	return versionsSDData, nil
}

func (vr verReader) readVersionDir(vDirPath string) (servExtData, error) {
	sData := servExtData{}
	assets, err := ioutil.ReadDir(vDirPath)
	if err != nil {
		return sData, fmt.Errorf("read version folder error: %v", err)
	}
	for _, asset := range assets {
		if asset.IsDir() {
			continue
		}
		if asset.Name() == vr.descriptionFilename {
			name, description, err := vr.readVersionDescriptionFile(filepath.Join(vDirPath, asset.Name()))
			if err != nil {
				return sData, err
			}
			sData.Name = strings.TrimSpace(name)
			sData.Description = strings.TrimSpace(description)
			continue
		}
		sData.Version = filepath.Base(vDirPath)
		sData.Assets = append(sData.Assets, struct {
			Filename string "json:\"filename\""
		}{Filename: asset.Name()})
	}
	return sData, nil
}

func (vr verReader) readVersionDescriptionFile(filepath string) (name string, description string, _ error) {
	desc, err := os.ReadFile(filepath)
	if err != nil {
		return "", "", fmt.Errorf("reading description error: %v", err)
	}
	if delimIndex := bytes.Index(desc, []byte(vr.descriptionNameSeparator)); delimIndex != -1 {
		return string(desc[0:delimIndex]), string(desc[delimIndex+len(vr.descriptionNameSeparator):]), nil
	}
	return "", string(desc), nil
}
