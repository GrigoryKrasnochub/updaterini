package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/GrigoryKrasnochub/updaterini"
	"github.com/urfave/cli/v2"
)

const descriptionNameSeparator = "\\\\//"
const descriptionFilename = "description.txt"
const outputFilename = "serv_update.json"

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
				Value:   descriptionFilename,
				Usage: "filename in version folder, that contains version description, " +
					"will be parsed and skipped in assets. Parsed description will be used in result json file",
			},
			&cli.StringFlag{
				Name:    "descNameSeparator",
				Aliases: []string{"s"},
				Value:   descriptionNameSeparator,
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
				outputFilepath, err = os.Executable()
				if err != nil {
					return err
				}
				outputFilepath = filepath.Join(filepath.Dir(outputFilepath), outputFilename)
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

func (vr verReader) readVersionsDir() ([]updaterini.ServData, error) {
	versions, err := ioutil.ReadDir(vr.versionsDir)
	versionsSDData := make([]updaterini.ServData, 0)
	if err != nil {
		return nil, fmt.Errorf("read versions dir error: %v", err)
	}
	for _, version := range versions {
		if !version.IsDir() {
			continue
		}
		_, err = updaterini.ParseVersion(version.Name())
		if err != nil {
			return nil, fmt.Errorf("parse version folder name error (incorrect version): %v", err)
		}
		sData, err := vr.readVersionDir(filepath.Join(vr.versionsDir, version.Name()))
		if err != nil {
			return nil, err
		}
		sData.Version = version.Name()
		versionsSDData = append(versionsSDData, sData)
	}
	return versionsSDData, nil
}

func (vr verReader) readVersionDir(vDirPath string) (updaterini.ServData, error) {
	sData := updaterini.ServData{}
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
		sData.Assets = append(sData.Assets, struct{ Filename string }{Filename: asset.Name()})
	}
	return sData, nil
}

func (vr verReader) readVersionDescriptionFile(filepath string) (name string, description string, _ error) {
	desc, err := os.ReadFile(filepath)
	if err != nil {
		return "", "", fmt.Errorf("reading description error: %v", err)
	}
	descRune := []rune(string(desc))
	delimRune := []rune(vr.descriptionNameSeparator)
	delimLen := len(delimRune)
	delimSymCounter := 0
	lastDelimIndex := 0
	for i, sym := range descRune {
		if sym == delimRune[delimSymCounter] {
			delimSymCounter++
			lastDelimIndex = i
			if delimSymCounter == delimLen {
				return string(descRune[0 : lastDelimIndex-delimLen+1]), string(descRune[lastDelimIndex+1:]), nil
			}
		}
	}
	return "", string(descRune), nil
}
