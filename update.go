package updaterini

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

var (
	ErrorFailUpdateRollback = errors.New("error. update rollback failed")
)

const OldVersionReplacedFilesExtension = ".old"
const VersionReplacedAndRollbackExtensionDif = "est"
const OldVersionRollbackFilesExtension = OldVersionReplacedFilesExtension + VersionReplacedAndRollbackExtensionDif

/*
	USE CAREFULLY! Func will delete files by extension. (CHECK OldVersionReplacedFilesExtension)

	func delete only files which have old and new version. Example:

	/test.txt.old

	/test.txt

	/example.exe

	func delete test.txt.old
*/
func UnsafeDeletePreviousVersionFiles(dirPath string) error {
	uR, err := getUpdateResultByDirScan(dirPath)
	if err != nil {
		return err
	}

	return uR.DeletePreviousVersionFiles(DeleteModPureDelete)
}

/*
	USE CAREFULLY! If prev update is not delete func rename files by extension. (CHECK OldVersionReplacedFilesExtension)
*/

type rollbackResults updateResult

func RollbackUpdate(dirPath string) (*rollbackResults, error) {
	uR, err := getUpdateResultByDirScan(dirPath)
	if err != nil {
		return nil, err
	}
	type rollbackFile struct {
		fileName                  string
		replacedRenamedToRollback bool
		usualRenamedToReplaced    bool
		rollbackRenamedToUsual    bool
	}

	rbFiles := make([]rollbackFile, len(uR.updateFilesInfo))
	rollbackUpdateOnErr := func(updateErr error) error {
		if updateErr == nil {
			return nil
		}
		for _, rbFile := range rbFiles {
			if rbFile.rollbackRenamedToUsual {
				err = os.Rename(rbFile.fileName, rbFile.fileName+OldVersionRollbackFilesExtension)
				if err != nil {
					return ErrorFailUpdateRollback
				}
			}
			if rbFile.usualRenamedToReplaced {
				err = os.Rename(rbFile.fileName+OldVersionReplacedFilesExtension, rbFile.fileName)
				if err != nil {
					return ErrorFailUpdateRollback
				}
			}
			if rbFile.replacedRenamedToRollback {
				err = os.Rename(rbFile.fileName+OldVersionRollbackFilesExtension, rbFile.fileName+OldVersionReplacedFilesExtension)
				if err != nil {
					return ErrorFailUpdateRollback
				}
			}
		}
		return updateErr
	}

	for i, val := range uR.updateFilesInfo {
		rbFiles[i] = rollbackFile{fileName: val.fileName}

		// .old to .oldest
		err = os.Rename(val.fileName+OldVersionReplacedFilesExtension, val.fileName+OldVersionRollbackFilesExtension)
		err = rollbackUpdateOnErr(err)
		if err != nil {
			return nil, err
		}
		rbFiles[i].replacedRenamedToRollback = true

		// usual to .old
		err = os.Rename(val.fileName, val.fileName+OldVersionReplacedFilesExtension)
		err = rollbackUpdateOnErr(err)
		if err != nil {
			return nil, err
		}
		rbFiles[i].usualRenamedToReplaced = true

		// .old to usual
		err = os.Rename(val.fileName+OldVersionRollbackFilesExtension, val.fileName)
		err = rollbackUpdateOnErr(err)
		if err != nil {
			return nil, err
		}
		rbFiles[i].rollbackRenamedToUsual = true
	}
	rbRes := rollbackResults(*uR)
	return &rbRes, nil
}

/*
	check dock in DeletePreviousVersionFiles func
*/
func (rbRes *rollbackResults) DeleteLoadedVersionFiles(mod DeleteMode, params ...interface{}) error {
	uRes := updateResult(*rbRes)
	return uRes.DeletePreviousVersionFiles(mod, params)
}

/*
	looking for new version in defined sources
*/
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

type updateResult struct {
	updateFilesInfo []updateFile
	updateDir       string
}

/*
	Load Files -> Check hash -> doBeforeUpdate() ->
	get file names from getReplacedFileName function, safe replace it if file exist in folder
	(curAppDir or cur exec file folder on empty string).
	Do rollback on any trouble
*/
func (uc *UpdateConfig) DoUpdate(ver Version, curAppDir string, getReplacedFileName func(loadedFilename string) (string, error), doBeforeUpdate func() error) (*updateResult, error) {
	if curAppDir == "" {
		exePath, err := os.Executable()
		if err != nil {
			return nil, err
		}
		curAppDir = filepath.Dir(exePath)
	}
	updateTempDir, err := os.MkdirTemp("", "update-*")
	if err != nil {
		return nil, err
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
		return nil, err
	}

	// TODO check files hash

	err = doBeforeUpdate()
	if err != nil {
		return nil, err
	}

	// replace files

	rollbackUpdateOnErr := func(updateErr error) error {
		if updateErr == nil {
			return nil
		}
		rollbackErr := rollbackUpdatedFiles(curAppDir, updateFilesInfo, OldVersionReplacedFilesExtension, false)
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
				return nil, err
			}
			updateFilesInfo[i].curFileRenamed = true
		}

		// move new file to dir
		err = os.Rename(updateFilesInfo[i].tmpFileName, curFilepath)
		err = rollbackUpdateOnErr(err)
		if err != nil {
			return nil, err
		}
		updateFilesInfo[i].updateFileMovedToDir = true
	}

	return &updateResult{
		updateFilesInfo: updateFilesInfo,
		updateDir:       curAppDir,
	}, nil
}

func (uR *updateResult) RollbackChanges() error {
	return rollbackUpdatedFiles(uR.updateDir, uR.updateFilesInfo, OldVersionReplacedFilesExtension, true)
}

type DeleteMode int

const (
	DeleteModPureDelete  DeleteMode = iota // Just delete files, can't delete files, which now are used or executed in Windows OS
	DeleteModKillProcess                   // successfully delete all prev version files, even if they are used by current process (for all os) after successful delete KILL current process (stop on err, no rollback)
	DeleteModRerunExec                     // successfully delete all prev version files, even if they are used by current process (for all os) after successful delete RUN exe (stop on err, no rollback)
)

/*
	Delete prev version files, choose delete type based on your purpose

	DeleteModPureDelete no params

	DeleteModKillProcess no params

	DeleteModRerunExec	use params to set executable file call args
*/
func (uR *updateResult) DeletePreviousVersionFiles(mode DeleteMode, params ...interface{}) error {
	switch mode {
	case DeleteModPureDelete:
		for _, file := range uR.updateFilesInfo {
			if !file.curFileRenamed {
				continue
			}
			curFilepath := filepath.Join(uR.updateDir, file.fileName)
			err := os.Remove(curFilepath + OldVersionReplacedFilesExtension)
			if err != nil {
				return err
			}
		}
	case DeleteModKillProcess:
		err := uR.deletePrevVersionFiles()
		if err != nil {
			return err
		}
		os.Exit(1)
	case DeleteModRerunExec:
		err := uR.deletePrevVersionFiles()
		if err != nil {
			return err
		}
		var exeArgs []string
		if params != nil {
			for _, param := range params {
				switch param.(type) {
				case string:
					exeArgs = append(exeArgs, param.(string))
				case []string:
					exeArgs = append(exeArgs, param.([]string)...)
				}
			}

		}
		err = uR.RerunExe(exeArgs)
		if err != nil {
			return err
		}
		os.Exit(1)
	}

	return nil
}

func (uR *updateResult) RerunExe(exeArgs []string) error { // TODO delete methode?
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(executable, exeArgs...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	err = cmd.Start()
	return err
}

/*
	for Windows OS executable should be stopped! USE ONLY WITH cur executable file stops functions
*/
func (uR *updateResult) deletePrevVersionFiles() error {
	var err error

	switch runtime.GOOS {
	case "windows":
		var errFiles []string
		for _, file := range uR.updateFilesInfo {
			if !file.curFileRenamed {
				continue
			}
			fPath := filepath.Join(uR.updateDir, file.fileName+OldVersionReplacedFilesExtension)
			err := os.Remove(fPath)
			if err != nil {
				errFiles = append(errFiles, fPath)
			}
		}
		var sI syscall.StartupInfo
		var pI syscall.ProcessInformation
		argv, tErr := syscall.UTF16PtrFromString(os.Getenv("windir") + "\\system32\\cmd.exe /C del " + strings.Join(errFiles, ", "))
		if tErr != nil {
			err = tErr
			break
		}
		err = syscall.CreateProcess(
			nil,
			argv,
			nil,
			nil,
			true,
			0,
			nil,
			nil,
			&sI,
			&pI)
	default:
		err = uR.DeletePreviousVersionFiles(DeleteModPureDelete)
	}

	return err
}

func getUpdateResultByDirScan(dirPath string) (*updateResult, error) {
	if dirPath == "" {
		dirPath = "."
	}
	type file struct {
		path      string
		hasOldVer bool
		hasNewVer bool
	}
	var files map[string]*file
	err := filepath.Walk(dirPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			fName := strings.TrimRight(info.Name(), OldVersionReplacedFilesExtension)
			if _, ok := files[fName]; !ok {
				files[fName] = &file{
					path: filepath.Dir(path),
				}
			}
			isOld := strings.HasSuffix(fName, OldVersionReplacedFilesExtension)
			if isOld {
				files[fName].hasOldVer = true
			} else {
				files[fName].hasNewVer = true
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	var updFileInfo []updateFile
	for fName, val := range files {
		if val.hasOldVer && val.hasNewVer {
			updFileInfo = append(updFileInfo, updateFile{
				fileName:             fName,
				tmpFileName:          "",
				curFileRenamed:       true,
				updateFileMovedToDir: true,
			})
		}
	}
	updRes := updateResult{
		updateFilesInfo: updFileInfo,
		updateDir:       dirPath,
	}
	return &updRes, nil
}

func rollbackUpdatedFiles(currentApplicationDir string, updateFiles []updateFile, preVersionExtension string, showErr bool) (err error) {
	defer func() {
		if !showErr && err != nil {
			err = ErrorFailUpdateRollback
		}
	}()
	for _, file := range updateFiles {
		if !file.curFileRenamed && !file.updateFileMovedToDir {
			continue
		}
		curFilepath := filepath.Join(currentApplicationDir, file.fileName)
		if file.updateFileMovedToDir {
			err := os.Remove(curFilepath)
			if err != nil {
				return err
			}
		}
		if file.curFileRenamed {
			err := os.Rename(curFilepath+preVersionExtension, curFilepath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
