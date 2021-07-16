package updaterini

import (
	"io"
	"os"
	"path/filepath"
)

func (uc *UpdateConfig) CheckForUpdates() (*version, error) {
	var versions []version
	for _, source := range uc.Sources {
		sVersions, err := source.getSourceVersions(uc.ApplicationConfig)
		if uc.ApplicationConfig.ShowPrepareVersionErr && err != nil {
			return nil, err
		}
		versions = append(versions, sVersions...)
	}
	ver, _ := getLatestVersion(uc.ApplicationConfig, versions)
	return ver, nil
}

/*
	For Debug purpose

	Load version update files to dirPath dir

	Empty string dirPath for place file near to executable file
*/
func (uc *UpdateConfig) LoadFilesToDir(ver version, dirPath string) error {
	err := ver.getAssetsFilesContent(uc.ApplicationConfig, func(reader io.Reader, filename string) error {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		if dirPath == "" {
			dirPath = filepath.Dir(exePath)
		}
		file, err := os.Create(filepath.Join(dirPath, filename))
		if err != nil {
			return err
		}
		defer func() {
			tempErr := file.Close()
			if err == nil {
				err = tempErr
			}
		}()
		_, err = io.Copy(file, reader)
		return err
	})
	return err
}
