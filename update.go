package updaterini

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrorFailUpdateRollback = errors.New("error. update rollback failed")

const oldVersionReplacedFilesExtension = ".old"
const versionReplacedAndRollbackExtensionDif = "est"
const oldVersionRollbackFilesExtension = oldVersionReplacedFilesExtension + versionReplacedAndRollbackExtensionDif

/*
	looking for new version in defined sources
*/
func (uc *UpdateConfig) CheckAllSourcesForUpdates() (Version, SourceCheckStatus) {
	var versions []Version
	var checkStatus SourceCheckStatus
	for _, source := range uc.Sources {
		sVersions, srcStatus := source.getSourceVersions(uc.ApplicationConfig)
		checkStatus.SourcesStatuses = append(checkStatus.SourcesStatuses, srcStatus)
		if srcStatus.Status == CheckFailure {
			continue
		}
		versions = append(versions, sVersions...)
	}
	checkStatus.updateSourceCheckStatus()
	ver := getLatestVersion(uc.ApplicationConfig, versions)
	return ver, checkStatus
}

/*
	looking for new version in defined sources. First source response with Ok code and any versions (even nil) will stop any other attempt to check other sources
*/
func (uc *UpdateConfig) CheckForUpdates() (Version, SourceCheckStatus) {
	var checkStatus SourceCheckStatus
	for _, source := range uc.Sources {
		sVersion, srcStatus := source.getSourceVersions(uc.ApplicationConfig)
		checkStatus.SourcesStatuses = append(checkStatus.SourcesStatuses, srcStatus)
		if srcStatus.Status == CheckFailure {
			continue
		}
		version := getLatestVersion(uc.ApplicationConfig, sVersion)
		checkStatus.updateSourceCheckStatus()
		return version, checkStatus
	}
	checkStatus.updateSourceCheckStatus()
	return nil, checkStatus
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
	err := ver.getAssetsFilesContent(uc.ApplicationConfig, func(reader io.Reader, filename string, _ int) (err error) {
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

const ReplacementFileInfoUseDefaultOrExistedFilePerm = 9999
const ReplacementFileDefaultMode = fs.FileMode(0644)

type ReplacementFile struct {
	FileName           string
	Mode               fs.FileMode // use ReplacementFileInfoUseDefaultOrExistedFilePerm to set default ReplacementFileDefaultMode or if file already exist, existed file permission
	PreventUnpacking   bool        // is archive shouldn't be unpacked
	PreventFileLoading bool        // is file should be skipped during update
}

type updateFile struct {
	replacement ReplacementFile // file after replace

	tmpFileName string

	curFileMode  fs.FileMode
	curFileOwner int
	curFileGroup int

	curFileRenamed        bool // is oldVersionReplacedFilesExtension attached to actual file
	replacementMovedToDir bool // is replacement file moved to dir
}

type UpdateResult struct {
	updateFilesInfo []updateFile
	updateDir       string
	curExeFilePath  string // for Linux rerun after update
}

/*
	Load Files -> Check hash -> doBeforeUpdate() ->
	get file names from getReplacementFileInfo function, safe replace it if file exist in folder
	(curAppDir or cur exec file folder on empty string).
	Do rollback on any trouble
*/
func (uc *UpdateConfig) DoUpdate(ver Version, curAppDir string, getReplacementFileInfo func(loadedFilename string) (ReplacementFile, error), doBeforeUpdate func() error) (_ UpdateResult, err error) {
	exePath, err := os.Executable()
	if err != nil {
		return UpdateResult{}, err
	}
	if curAppDir == "" {
		curAppDir = filepath.Dir(exePath)
	}
	updateTempDir, err := os.MkdirTemp("", "update-*")
	if err != nil {
		return UpdateResult{}, err
	}
	defer func() {
		tempErr := os.RemoveAll(updateTempDir)
		if err == nil {
			err = tempErr
		}
	}()

	// load all files

	repFileInfo := make(map[int]ReplacementFile)
	err = ver.removeAssets(func(filename string, id int) (bool, error) {
		replacementFileInfo, err := getReplacementFileInfo(filename)
		if err != nil {
			return false, err
		}
		if replacementFileInfo.PreventFileLoading {
			return true, nil
		}
		repFileInfo[id] = replacementFileInfo
		return false, nil
	})
	if err != nil {
		return UpdateResult{}, err
	}

	updateFilesInfo := make([]updateFile, 0)
	err = ver.getAssetsFilesContent(uc.ApplicationConfig, func(reader io.Reader, filename string, id int) (err error) {
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
			replacement:           repFileInfo[id],
			tmpFileName:           tFile.Name(),
			curFileRenamed:        false,
			replacementMovedToDir: false,
		})
		return err
	})
	repFileInfo = nil
	if err != nil {
		return UpdateResult{}, err
	}

	err = doBeforeUpdate()
	if err != nil {
		return UpdateResult{}, err
	}

	// replace files

	rollbackUpdateOnErr := func(updateErr error) error {
		if updateErr == nil {
			return nil
		}
		rollbackErr := rollbackUpdatedFiles(curAppDir, updateFilesInfo, oldVersionReplacedFilesExtension, true)
		if rollbackErr != nil {
			return fmt.Errorf("rollback error: %v update error: %v", rollbackErr, updateErr)
		}
		return updateErr
	}

	for i := range updateFilesInfo {
		curFilepath := filepath.Join(curAppDir, updateFilesInfo[i].replacement.FileName)
		fInfo, err := os.Stat(curFilepath)

		// rename old file is exist
		if err == nil {
			if fInfo.IsDir() {
				continue
			}
			err = os.Rename(curFilepath, curFilepath+oldVersionReplacedFilesExtension)
			err = rollbackUpdateOnErr(err)
			if err != nil {
				return UpdateResult{}, err
			}
			updateFilesInfo[i].curFileRenamed = true
			updateFilesInfo[i].curFileMode = fInfo.Mode().Perm()
			updateFilesInfo[i].fillFileOwnerInfo(fInfo)
		}

		// move new file to dir
		err = os.Rename(updateFilesInfo[i].tmpFileName, curFilepath)
		err = rollbackUpdateOnErr(err)
		if err != nil {
			return UpdateResult{}, err
		}
		fMode := updateFilesInfo[i].replacement.Mode
		if fMode == ReplacementFileInfoUseDefaultOrExistedFilePerm {
			if updateFilesInfo[i].curFileRenamed {
				fMode = updateFilesInfo[i].curFileMode
			} else {
				fMode = ReplacementFileDefaultMode
			}
		}
		err = os.Chmod(curFilepath, fMode)
		err = rollbackUpdateOnErr(err)
		if err != nil {
			return UpdateResult{}, err
		}
		if updateFilesInfo[i].curFileRenamed && (updateFilesInfo[i].curFileOwner != -1 || updateFilesInfo[i].curFileGroup != -1) {
			err = os.Chown(curFilepath, updateFilesInfo[i].curFileOwner, updateFilesInfo[i].curFileGroup)
			err = rollbackUpdateOnErr(err)
		}
		if err != nil {
			return UpdateResult{}, err
		}
		updateFilesInfo[i].replacementMovedToDir = true
	}

	return UpdateResult{
		updateFilesInfo: updateFilesInfo,
		updateDir:       curAppDir,
		curExeFilePath:  exePath,
	}, nil
}

func (uR *UpdateResult) RollbackChanges() error {
	return rollbackUpdatedFiles(uR.updateDir, uR.updateFilesInfo, oldVersionReplacedFilesExtension, true)
}

type DeleteMode int

const (
	// DeleteModPureDelete just delete files, can't delete files, which now are used or executed in Windows OS
	DeleteModPureDelete DeleteMode = iota
	// DeleteModKillProcess successfully delete all prev version files, even if they are used by current process (for all os)
	// after successful delete KILL current process (stop on err, no rollback)
	DeleteModKillProcess
	// DeleteModRerunExec successfully delete all prev version files, even if they are used by current process (for all os)
	// after successful delete RUN exe (stop on err, no rollback)
	DeleteModRerunExec
)

/*
	Delete prev version files, choose to delete type based on your purpose

	DeleteModPureDelete no params

	DeleteModKillProcess no params

	DeleteModRerunExec	use params to set executable file call args
*/
func (uR *UpdateResult) DeletePreviousVersionFiles(mode DeleteMode, params ...interface{}) error {
	switch mode {
	case DeleteModPureDelete:
		for _, file := range uR.updateFilesInfo {
			if !file.curFileRenamed {
				continue
			}
			curFilepath := filepath.Join(uR.updateDir, file.replacement.FileName)
			err := os.Remove(curFilepath + oldVersionReplacedFilesExtension)
			if err != nil {
				return err
			}
		}
	case DeleteModKillProcess:
		err := uR.deletePrevVersionFiles()
		if err != nil {
			return err
		}
		os.Exit(0)
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
		os.Exit(0)
	}
	return nil
}

func (uR *UpdateResult) RerunExe(exeArgs []string) error {
	cmd := exec.Command(uR.curExeFilePath, exeArgs...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd.Start()
}

/*
	USE CAREFULLY! Func will delete files by extension. (CHECK oldVersionReplacedFilesExtension)

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

type RollbackResults UpdateResult

/*
	USE CAREFULLY! If prev update is not deleted func rename files by extension. (CHECK oldVersionReplacedFilesExtension)
*/
func UnsafeRollbackUpdate(dirPath string) (*RollbackResults, error) {
	uR, err := getUpdateResultByDirScan(dirPath)
	if err != nil {
		return nil, err
	}

	type rollbackFile struct {
		filePath                  string
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
				err = os.Rename(rbFile.filePath, rbFile.filePath+oldVersionRollbackFilesExtension)
				if err != nil {
					return ErrorFailUpdateRollback
				}
			}
			if rbFile.usualRenamedToReplaced {
				err = os.Rename(rbFile.filePath+oldVersionReplacedFilesExtension, rbFile.filePath)
				if err != nil {
					return ErrorFailUpdateRollback
				}
			}
			if rbFile.replacedRenamedToRollback {
				err = os.Rename(rbFile.filePath+oldVersionRollbackFilesExtension, rbFile.filePath+oldVersionReplacedFilesExtension)
				if err != nil {
					return ErrorFailUpdateRollback
				}
			}
		}
		return updateErr
	}

	for i, val := range uR.updateFilesInfo {
		rbFiles[i] = rollbackFile{filePath: filepath.Join(uR.updateDir, val.replacement.FileName)}

		// .old to .oldest
		err = os.Rename(rbFiles[i].filePath+oldVersionReplacedFilesExtension, rbFiles[i].filePath+oldVersionRollbackFilesExtension)
		err = rollbackUpdateOnErr(err)
		if err != nil {
			return nil, err
		}
		rbFiles[i].replacedRenamedToRollback = true

		// usual to .old
		err = os.Rename(rbFiles[i].filePath, rbFiles[i].filePath+oldVersionReplacedFilesExtension)
		err = rollbackUpdateOnErr(err)
		if err != nil {
			return nil, err
		}
		rbFiles[i].usualRenamedToReplaced = true

		// .oldest to usual
		err = os.Rename(rbFiles[i].filePath+oldVersionRollbackFilesExtension, rbFiles[i].filePath)
		err = rollbackUpdateOnErr(err)
		if err != nil {
			return nil, err
		}
		rbFiles[i].rollbackRenamedToUsual = true
	}
	rbRes := RollbackResults(uR)
	return &rbRes, nil
}

/*
	check documentation in DeletePreviousVersionFiles func
*/
func (rbRes *RollbackResults) DeleteLoadedVersionFiles(mod DeleteMode, params ...interface{}) error {
	uRes := UpdateResult(*rbRes)
	return uRes.DeletePreviousVersionFiles(mod, params)
}

/*
	don't fills files mode
*/
func getUpdateResultByDirScan(dirPath string) (UpdateResult, error) {
	if dirPath == "" {
		dirPath = "."
	}
	type file struct {
		hasOldVer   bool
		hasNewVer   bool
		oldVerFMode fs.FileMode
	}
	files := make(map[string]*file)
	err := filepath.Walk(dirPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			relFilePath, err := filepath.Rel(dirPath, path)
			if err != nil {
				return err
			}
			relFilePath = strings.TrimSuffix(relFilePath, oldVersionReplacedFilesExtension)
			if _, ok := files[relFilePath]; !ok {
				files[relFilePath] = &file{}
			}
			isOld := strings.HasSuffix(info.Name(), oldVersionReplacedFilesExtension)
			if isOld {
				files[relFilePath].hasOldVer = true
				files[relFilePath].oldVerFMode = info.Mode()
			} else {
				files[relFilePath].hasNewVer = true
			}
			return nil
		})
	if err != nil {
		return UpdateResult{}, err
	}
	var updFileInfo []updateFile
	for fName, val := range files {
		if val.hasOldVer && val.hasNewVer {
			updFileInfo = append(updFileInfo, updateFile{
				replacement: ReplacementFile{
					FileName: fName,
					Mode:     ReplacementFileInfoUseDefaultOrExistedFilePerm,
				},
				tmpFileName:           "",
				curFileMode:           val.oldVerFMode,
				curFileRenamed:        true,
				replacementMovedToDir: true,
			})
		}
	}
	updRes := UpdateResult{
		updateFilesInfo: updFileInfo,
		updateDir:       dirPath,
	}
	return updRes, nil
}

func rollbackUpdatedFiles(currentApplicationDir string, updateFiles []updateFile, preVersionExtension string, showErr bool) (err error) {
	defer func() {
		if !showErr && err != nil {
			err = ErrorFailUpdateRollback
		}
	}()
	for _, file := range updateFiles {
		if !file.curFileRenamed && !file.replacementMovedToDir {
			continue
		}
		curFilepath := filepath.Join(currentApplicationDir, file.replacement.FileName)
		if file.replacementMovedToDir {
			err = os.Remove(curFilepath)
			if err != nil {
				return
			}
		}
		if file.curFileRenamed {
			err = os.Rename(curFilepath+preVersionExtension, curFilepath)
			if err != nil {
				return
			}
		}
	}
	return
}
