package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/GrigoryKrasnochub/updaterini"
)

func main() {
	updateExeFile()
}

/*
	If you are Windows user do go build in examples dir

	examples.exe would be updated to last release in updaterini_example rep
*/
func updateExeFile() {
	appConf, err := updaterini.NewApplicationConfig("1.0.0", []updaterini.Channel{updaterini.NewReleaseChannel(true)}, nil)
	if err != nil {
		panic(err)
	}
	update := updaterini.UpdateConfig{
		ApplicationConfig: appConf,
		Sources: []updaterini.UpdateSource{
			&updaterini.UpdateSourceServer{
				UpdatesMapURL: "http://unexistedServer/example.json",
			},
			&updaterini.UpdateSourceServer{
				UpdatesMapURL: "http://example/example.json",
			},
			&updaterini.UpdateSourceGitRepo{
				UserName: "GrigoryKrasnochub",
				RepoName: "updaterini_example",
			},
		},
	}

	version := update.CheckForUpdatesWithErrCallback(func(err error, source updaterini.UpdateSource, sourceIndex int) error {
		switch source.(type) {
		case *updaterini.UpdateSourceGitRepo:
			fmt.Println("GitSourceErr ", err)
		case *updaterini.UpdateSourceServer:
			fmt.Println("ServerSourceErr ", err)
		}
		return nil
	})

	if version != nil {
		fmt.Println("Start Update!")
		counter := 0
		updateResult, err := update.DoUpdate(version, "", func(loadedFilename string) (string, error) {
			if strings.HasSuffix(loadedFilename, ".exe") {
				exec, _ := os.Executable()
				return filepath.Base(exec), nil // current exe file will be replaced
			}
			counter++
			return fmt.Sprintf("new file %d", counter), nil
		}, func() error {
			return nil
		})
		if err != nil {
			panic(err)
		}
		err = updateResult.DeletePreviousVersionFiles(updaterini.DeleteModRerunExec, nil)
		if err != nil {
			panic(err)
		}
		fmt.Println("Update DONE!")
	} else {
		fmt.Println("No versions for update")
	}
}

func simpleVersionFileLoad() {
	appConf, err := updaterini.NewApplicationConfig("1.0.0", []updaterini.Channel{updaterini.NewReleaseChannel(true)}, nil)
	if err != nil {
		panic(err)
	}
	update := updaterini.UpdateConfig{
		ApplicationConfig: appConf,
		Sources: []updaterini.UpdateSource{
			&updaterini.UpdateSourceGitRepo{
				UserName: "GrigoryKrasnochub",
				RepoName: "updaterini_example",
			},
		},
	}

	version, err := update.CheckForUpdates()
	if err != nil {
		panic(err)
	}

	if version != nil {
		fmt.Println("Start Update!")

		// load file to your build dir
		err = update.LoadFilesToDir(version, "")
		if err != nil {
			panic(err)
		}
		fmt.Println("Update DONE!")
	} else {
		fmt.Println("No versions for update")
	}
}
