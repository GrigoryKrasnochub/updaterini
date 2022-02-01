package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/urfave/cli/v2"
)

const descriptionNameSeparator = "====="
const descriptionFilename = "description.txt"
const outputFilename = "serv_update.json"

func main() {
	app := &cli.App{
		Name:  "updaterini",
		Usage: "utils for updaterini update library",
		Commands: []*cli.Command{
			{
				Name:    "sergen",
				Aliases: []string{"sgen"},
				Usage:   "generate json description file for server update source of updaterini library",
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
						Value:   outputFilename,
						Usage:   "path to place generator output file",
					},
				},
				Action: func(context *cli.Context) error {
					var err error
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

					outputFilepath := context.Path("outputFilepath")
					err = ioutil.WriteFile(outputFilepath, jsonVersions, 0644)
					if err != nil {
						return fmt.Errorf("save output file error: %v", err)
					}

					fmt.Printf("Description created Sucessfully! Versions counter: %d", len(versions))
					return nil
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}
