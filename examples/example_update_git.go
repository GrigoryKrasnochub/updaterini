package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/GrigoryKrasnochub/updaterini"
)

func main() {
	updateExeFile()
}

/*
	If you are Windows/Linux user do go build in examples dir

	examples would be updated to last release in updaterini_example rep
*/
func updateExeFile() {
	appConf, err := updaterini.NewApplicationConfig("1.0.0", []updaterini.Channel{updaterini.NewReleaseChannel(true)}, nil)
	appConf.ShowPrepareVersionErr = true
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
				UserName:            "GrigoryKrasnochub",
				RepoName:            "updaterini_example",
				PersonalAccessToken: "",
			},
		},
	}

	version, checkStatus := update.CheckForUpdates()
	if checkStatus.Status != updaterini.CheckFailure {
		fmt.Println("Update errors:")
		for _, srcStatus := range checkStatus.SourcesStatuses {
			if srcStatus.Status != updaterini.CheckSuccess {
				switch srcStatus.Source.(type) {
				case *updaterini.UpdateSourceGitRepo:
					src, _ := srcStatus.Source.(*updaterini.UpdateSourceGitRepo)
					fmt.Printf("Source type: %s;\n Repo: %s;\n Erros: %v;\n", srcStatus.Source.SourceLabel(), src.RepoName, srcStatus.Errors)
				case *updaterini.UpdateSourceServer:
					src, _ := srcStatus.Source.(*updaterini.UpdateSourceServer)
					fmt.Printf("Source type: %s;\n URL: %s;\n Erros: %v;\n", srcStatus.Source.SourceLabel(), src.UpdatesMapURL, srcStatus.Errors)
				}
			}
		}
	} else {
		panic("Update failed")
	}

	if version != nil {
		fmt.Println("Start Update!")
		counter := 0
		updateResult, err := update.DoUpdate(version, "", func(loadedFilename string) (updaterini.ReplacementFile, error) {
			if counter == 0 {
				exec, _ := os.Executable()
				return updaterini.ReplacementFile{
					FileName: filepath.Base(exec),
					Mode:     updaterini.ReplacementFileInfoUseDefaultOrExistedFilePerm,
				}, nil // current exe file will be replaced
			}
			counter++
			return updaterini.ReplacementFile{
				FileName: fmt.Sprintf("new file %d", counter),
				Mode:     updaterini.ReplacementFileInfoUseDefaultOrExistedFilePerm,
			}, nil
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

	version, checkStatus := update.CheckForUpdates()
	if checkStatus.Status == updaterini.CheckFailure {
		panic(checkStatus.Status)
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
