package main

import (
	"fmt"

	"github.com/GrigoryKrasnochub/updaterini"
)

func main() {
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
		//load file to your build dir
		err = update.LoadFilesToDir(*version, "")
		if err != nil {
			return
		}
		fmt.Println("Update DONE!")
	} else {
		fmt.Println("No versions for update")
	}
}
