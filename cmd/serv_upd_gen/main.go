package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/GrigoryKrasnochub/updaterini"
	"github.com/blang/semver/v4"
	"github.com/urfave/cli/v2"
)

const DescriptionNameSeparator = "==============="
const DescriptionFilename = "description.txt"
const OutputFilename = "serv_update.json"

func main() {
	app := &cli.App{
		Name:  "serve update description generator",
		Usage: "generate json description file for serve update source of updaterini library",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "url",
				Aliases:  []string{"u"},
				Required: true,
				Usage:    "base url for all versions, (base url + version folder + asset file name) is used by library to load asset from server",
			},
			&cli.StringFlag{
				Name:    "descFilename",
				Aliases: []string{"d"},
				Value:   DescriptionFilename,
				Usage: "filename in version folder, that contains version description, " +
					"will be parsed and skipped in assets. Parsed description will be used in result json file",
			},
			&cli.StringFlag{
				Name:    "descNameSeparator",
				Aliases: []string{"s"},
				Value:   DescriptionNameSeparator,
				Usage:   "separator that separate version name from version description in version description file",
			},
			&cli.PathFlag{
				Name:     "inputDir",
				Aliases:  []string{"id"},
				Required: true,
				Usage:    "path to versions folder",
			},
			&cli.PathFlag{
				Name:    "outputFilepath",
				Aliases: []string{"of"},
				Usage:   "path to place generator output file",
			},
		},
		Action: func(context *cli.Context) error {
			var err error
			outputFilepath := context.Path("outputFilepath")
			if outputFilepath == "" {
				outputFilepath, err = os.Getwd()
				if err != nil {
					return err
				}
				outputFilepath = filepath.Join(outputFilepath, OutputFilename)
			}
			vReader := verReader{
				versionsDir:              context.Path("inputDir"),
				descriptionFilename:      context.String("descFilename"),
				descriptionNameSeparator: context.String("descNameSeparator"),
			}
			versions, err := vReader.readVersionsDir()
			if err != nil {
				return err
			}

			// sort versions asc

			sort.SliceStable(versions, func(i, j int) bool {
				return versions[i].version.LT(versions[i].version)
			})

			// fullfill versions info

			baseUrl := context.String("url")
			if !strings.HasSuffix(baseUrl, "/") {
				baseUrl += "/"
			}
			for i, ver := range versions {
				versions[i].VersionFolderUrl = baseUrl + ver.Version + "/"
			}

			jsonVersions, err := json.Marshal(versions)
			if err != nil {
				return err
			}
			err = ioutil.WriteFile(outputFilepath, jsonVersions, 0644)
			if err != nil {
				return fmt.Errorf("save output file error: %v", err)
			}
			fmt.Printf("Description created Sucessfully! Versions counter: %d", len(versions))
			return nil
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}

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
