package main

import (
	"fmt"
	"strings"

	"github.com/GrigoryKrasnochub/updaterini"
)

func main() {
	updateExeFile()
}

/*
	Do go build in examples dir

	empty.exe would be updated to last release in updaterini_example rep
*/
func updateExeFile() {
	appConf, err := updaterini.NewApplicationConfig("1.0.0", []updaterini.Channel{updaterini.GetReleaseChanel(true)})
	if err != nil {
		panic(err)
	}
	update := updaterini.UpdateConfig{
		ApplicationConfig: *appConf,
		Sources: []updaterini.UpdateSource{
			&updaterini.UpdateSourceGitRepo{
				UserName: "GrigoryKrasnochub",
				RepoName: "updaterini_example",
			},
		},
	}
	version, err := update.CheckForUpdates()
	if err != nil {
		fmt.Println("Ou, some critical err ", err)
	}
	if version != nil {
		fmt.Println("Start Update!")
		counter := 0
		err = update.DoUpdate(*version, "", func(loadedFilename string) (string, error) {
			if strings.HasSuffix(loadedFilename, ".exe") {
				return "empty.exe", nil
			}
			counter++
			return fmt.Sprintf("new file %d", counter), nil
		}, func() error {
			return nil
		})
		if err != nil {
			fmt.Println("Ou, some err ", err)
		}
		fmt.Println("Update DONE!")
	} else {
		fmt.Println("No versions for update")
	}
}

func simpleVersionFileLoad() {
	appConf, err := updaterini.NewApplicationConfig("1.0.0", []updaterini.Channel{updaterini.GetReleaseChanel(true)})
	if err != nil {
		panic(err)
	}
	update := updaterini.UpdateConfig{
		ApplicationConfig: *appConf,
		Sources: []updaterini.UpdateSource{
			&updaterini.UpdateSourceGitRepo{
				UserName: "GrigoryKrasnochub",
				RepoName: "updaterini_example",
			},
		},
	}
	version, err := update.CheckForUpdates()
	if err != nil {
		fmt.Println("Ou, some critical err ", err)
	}
	if version != nil {
		fmt.Println("Start Update!")

		// load file to your build dir
		err = update.LoadFilesToDir(*version, "")
		if err != nil {
			fmt.Println("Ou, some err ", err)
		}
		fmt.Println("Update DONE!")
	} else {
		fmt.Println("No versions for update")
	}
}
