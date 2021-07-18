package updaterini

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	ErrorFailUpdateRollback = errors.New("error. update rollback failed")
)

const OldVersionReplacedFilesExtension = ".old"

func (uc *UpdateConfig) CheckForUpdates() (*Version, error) {
	var versions []Version
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
func (uc *UpdateConfig) LoadFilesToDir(ver Version, dirPath string) error {
	if dirPath == "" {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		dirPath = filepath.Dir(exePath)
	}
	err := ver.getAssetsFilesContent(uc.ApplicationConfig, func(reader io.Reader, filename string) error {
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

type updateFile struct {
	fileName             string
	tmpFileName          string
	curFileRenamed       bool
	updateFileMovedToDir bool
}

/*
	Load Files -> Check hash -> doBeforeUpdate() ->
	get file names from getReplacedFileName function, safe replace it if file exist in executable file folder.
	Do rollback on any trouble
*/
func (uc *UpdateConfig) DoUpdate(ver Version, curAppDir string, getReplacedFileName func(loadedFilename string) (string, error), doBeforeUpdate func() error) error {
	if curAppDir == "" {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		curAppDir = filepath.Dir(exePath)
	}
	updateTempDir, err := os.MkdirTemp("", "update-*")
	if err != nil {
		return err
	}
	defer func() {
		tempErr := os.RemoveAll(updateTempDir)
		if err == nil {
			err = tempErr
		}
	}()

	// load all files

	updateFilesInfo := make([]updateFile, 0)
	err = ver.getAssetsFilesContent(uc.ApplicationConfig, func(reader io.Reader, filename string) error {
		curFileName, err := getReplacedFileName(filename)
		if err != nil {
			return err
		}
		tFile, err := os.CreateTemp(updateTempDir, fmt.Sprintf("update-file-*-%s", filename))
		if err != nil {
			return err
		}
		defer func() {
			tempErr := tFile.Close()
			if err == nil {
				err = tempErr
			}
		}()
		_, err = io.Copy(tFile, reader)
		updateFilesInfo = append(updateFilesInfo, updateFile{
			fileName:             curFileName,
			tmpFileName:          tFile.Name(),
			curFileRenamed:       false,
			updateFileMovedToDir: false,
		})
		return err
	})
	if err != nil {
		return err
	}

	// TODO check files hash

	err = doBeforeUpdate()
	if err != nil {
		return err
	}

	// replace files

	rollbackUpdateOnErr := func(updateErr error) error {
		if updateErr == nil {
			return nil
		}
		rollbackErr := rollbackUpdatedFiles(curAppDir, updateFilesInfo)
		if rollbackErr != nil {
			return rollbackErr
		}
		return updateErr
	}

	for i := range updateFilesInfo {
		curFilepath := filepath.Join(curAppDir, updateFilesInfo[i].fileName)
		fInfo, err := os.Stat(curFilepath)

		// rename old file is exist
		if err == nil {
			if fInfo.IsDir() {
				continue
			}
			err = os.Rename(curFilepath, curFilepath+OldVersionReplacedFilesExtension)
			err = rollbackUpdateOnErr(err)
			if err != nil {
				return err
			}
			updateFilesInfo[i].curFileRenamed = true
		}

		// move new file to dir
		err = os.Rename(updateFilesInfo[i].tmpFileName, curFilepath)
		err = rollbackUpdateOnErr(err)
		if err != nil {
			return err
		}
		updateFilesInfo[i].updateFileMovedToDir = true
	}

	return deletePreviousVersionFiles(curAppDir, updateFilesInfo) // TODO Windows can't delete function this way
}

func deletePreviousVersionFiles(currentApplicationDir string, updateFiles []updateFile) error {
	for _, file := range updateFiles {
		if !file.curFileRenamed {
			continue
		}
		curFilepath := filepath.Join(currentApplicationDir, file.fileName)
		err := os.Remove(curFilepath + OldVersionReplacedFilesExtension)
		if err != nil {
			return err
		}
	}
	return nil
}

func rollbackUpdatedFiles(currentApplicationDir string, updateFiles []updateFile) error {
	for _, file := range updateFiles {
		if !file.curFileRenamed && !file.updateFileMovedToDir {
			continue
		}
		curFilepath := filepath.Join(currentApplicationDir, file.fileName)
		if file.updateFileMovedToDir {
			err := os.Remove(curFilepath)
			if err != nil {
				return ErrorFailUpdateRollback
			}
		}
		if file.curFileRenamed {
			err := os.Rename(curFilepath+OldVersionReplacedFilesExtension, curFilepath)
			if err != nil {
				return ErrorFailUpdateRollback
			}
		}
	}
	return nil
}
