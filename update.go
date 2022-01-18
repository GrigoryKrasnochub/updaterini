package updaterini

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
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
	vfl := versionFilesLoader{
		version:      ver,
		updateConfig: uc,
		getReplacementFileInfo: func(loadedFilename string) (ReplacementFile, error) {
			return ReplacementFile{
				FileName:           loadedFilename,
				Mode:               ReplacementFileDefaultMode,
				PreventFileLoading: false,
			}, nil
		},
		destDir:             dirPath,
		createOrdinaryFiles: true,
	}
	assetsFilenames := vfl.version.getAssetsFilenames()
	_, err := vfl.loadUpdateFilesFromSource(assetsFilenames)
	return err
}

const ReplacementFileInfoUseDefaultOrExistedFilePerm = 9999
const ReplacementFileDefaultMode = fs.FileMode(0644)

type ReplacementFile struct {
	FileName           string
	fileDir            string      // rel path to dest dir
	Mode               fs.FileMode // use ReplacementFileInfoUseDefaultOrExistedFilePerm to set default ReplacementFileDefaultMode or if file already exist, existed file permission
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
		if err != nil && tempErr != nil {
			err = fmt.Errorf("%v; remove all assets temp files error: %v", err, tempErr)
		}
		if err == nil {
			err = tempErr
		}
	}()

	// load all files

	vfl := versionFilesLoader{
		version:                ver,
		updateConfig:           uc,
		getReplacementFileInfo: getReplacementFileInfo,
		destDir:                updateTempDir,
	}
	updateFilesInfo, err := vfl.loadVersionFiles()
	if err != nil {
		return UpdateResult{}, err
	}

	if len(updateFilesInfo) == 0 {
		err = errors.New("update error: no assets for update (loading of all assets was prevented)")
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

	uniqRelPaths := make(map[string]struct{}, 0)
	for i := range updateFilesInfo {
		curDirPath := filepath.Join(curAppDir, updateFilesInfo[i].replacement.fileDir)
		if updateFilesInfo[i].replacement.fileDir != "" && updateFilesInfo[i].replacement.fileDir != "." {
			if _, ok := uniqRelPaths[updateFilesInfo[i].replacement.fileDir]; !ok {
				err = os.MkdirAll(curDirPath, ReplacementFileDefaultMode)
				err = rollbackUpdateOnErr(err)
				if err != nil {
					return UpdateResult{}, err
				}
				uniqRelPaths[updateFilesInfo[i].replacement.fileDir] = struct{}{}
			}
		}
		curFilepath := filepath.Join(curDirPath, updateFilesInfo[i].replacement.FileName)
		fInfo, err1 := os.Stat(curFilepath)

		// rename old file is exist
		if err1 == nil {
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
	}, err
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
			curFilepath := filepath.Join(uR.updateDir, file.replacement.fileDir, file.replacement.FileName)
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
					FileName: filepath.Base(fName),
					fileDir:  filepath.Dir(fName),
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
		curFilepath := filepath.Join(currentApplicationDir, file.replacement.fileDir, file.replacement.FileName)
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

const ZipArchiveExtension = ".zip"

var TarGzArchiveExtensions = []string{".tgz", ".tar.gz"}

type versionFilesLoader struct {
	version                Version
	updateConfig           *UpdateConfig
	getReplacementFileInfo func(loadedFilename string) (ReplacementFile, error)
	destDir                string
	createOrdinaryFiles    bool // create temp files otherwise USE CAREFULLY files could replace each other in ordinary naming
}

func (vfl versionFilesLoader) loadVersionFiles() ([]updateFile, error) {
	assetsFilenames := vfl.version.getAssetsFilenames()
	archivesFilenames := make([]string, 0)
	for i, filename := range assetsFilenames {
		isArchive := false
		for _, ext := range append(TarGzArchiveExtensions, ZipArchiveExtension) {
			ok, err := filepath.Match("*"+ext, filename)
			if err != nil {
				return nil, err
			}
			if ok {
				isArchive = true
				break
			}
		}
		if isArchive {
			archivesFilenames = append(archivesFilenames, filename)
			continue
		}
		assetsFilenames[i-len(archivesFilenames)] = filename
	}
	assetsFilenames = assetsFilenames[:len(assetsFilenames)-len(archivesFilenames)]

	updateFilesInfo, err := vfl.loadUpdateFilesFromSource(assetsFilenames)
	if err != nil {
		return nil, err
	}
	archivesUpdateFilesInfo, err := vfl.loadUpdateFilesFromSourceArchives(archivesFilenames)
	if err != nil {
		return nil, err
	}
	return append(updateFilesInfo, archivesUpdateFilesInfo...), nil
}

func (vfl versionFilesLoader) loadUpdateFilesFromSourceArchives(archivesFilenames []string) ([]updateFile, error) {
	updateFilesInfo := make([]updateFile, 0)
	for _, archive := range archivesFilenames {
		fExt := filepath.Ext(archive)
		fPath, err := vfl.loadUpdateFileFromSource(archive)
		if err != nil {
			return nil, err
		}
		var ufi []updateFile
		if fExt == ZipArchiveExtension {
			ufi, err = vfl.unpackZipArchive(fPath)

		} else {
			ufi, err = vfl.unpackTarGzArchive(fPath)
		}
		if err != nil {
			return nil, err
		}
		updateFilesInfo = append(updateFilesInfo, ufi...)
	}
	return updateFilesInfo, nil
}

func (vfl versionFilesLoader) unpackZipArchive(archiveFPath string) ([]updateFile, error) {
	updateFilesInfo := make([]updateFile, 0)
	zR, err := zip.OpenReader(archiveFPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		zRCloseErr := zR.Close()
		if err != nil && zRCloseErr != nil {
			err = fmt.Errorf("%v; zip close error: %v", err, zRCloseErr)
		}
		if err == nil {
			err = zRCloseErr
		}
	}()

	for _, file := range zR.File {
		if file.FileInfo().IsDir() {
			continue
		}
		fName := filepath.Base(file.Name)
		replacementFileInfo, err1 := vfl.getReplacementFileInfo(fName)
		if err1 != nil || replacementFileInfo.PreventFileLoading {
			err = err1
			return nil, err
		}
		replacementFileInfo.fileDir = filepath.Dir(file.Name)

		zFReader, err1 := file.Open()
		if err1 != nil {
			err = err1
			return nil, err
		}
		tFName, err1 := vfl.writeTempFileToDir(zFReader, fName)
		if err1 != nil {
			err = err1
			return nil, err
		}
		updateFilesInfo = append(updateFilesInfo, updateFile{
			replacement:           replacementFileInfo,
			tmpFileName:           tFName,
			curFileRenamed:        false,
			replacementMovedToDir: false,
		})
	}

	return updateFilesInfo, nil
}

func (vfl versionFilesLoader) unpackTarGzArchive(archiveFPath string) ([]updateFile, error) {
	updateFilesInfo := make([]updateFile, 0)
	fR, err := os.Open(archiveFPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		fRCloseErr := fR.Close()
		if err != nil && fRCloseErr != nil {
			err = fmt.Errorf("%v; targz file close error: %v", err, fRCloseErr)
		}
		if err == nil {
			err = fRCloseErr
		}
	}()
	gzR, err := gzip.NewReader(fR)
	if err != nil {
		return nil, err
	}
	defer func() {
		gzRCloseErr := gzR.Close()
		if err != nil && gzRCloseErr != nil {
			err = fmt.Errorf("%v; targz archive close error: %v", err, gzRCloseErr)
		}
		if err == nil {
			err = gzRCloseErr
		}
	}()
	tR := tar.NewReader(gzR)

	for {
		hdr, err1 := tR.Next()
		if err1 == io.EOF {
			break
		}
		if err1 != nil {
			err = err1
			return nil, err
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		fName := filepath.Base(hdr.Name)
		replacementFileInfo, err1 := vfl.getReplacementFileInfo(fName)
		if err1 != nil || replacementFileInfo.PreventFileLoading {
			err = err1
			return nil, err
		}
		replacementFileInfo.fileDir = filepath.Dir(hdr.Name)

		tFName, err1 := vfl.writeTempFileToDir(tR, fName)
		if err1 != nil {
			err = err1
			return nil, err
		}
		updateFilesInfo = append(updateFilesInfo, updateFile{
			replacement:           replacementFileInfo,
			tmpFileName:           tFName,
			curFileRenamed:        false,
			replacementMovedToDir: false,
		})
	}

	return updateFilesInfo, nil
}

func (vfl versionFilesLoader) loadUpdateFilesFromSource(assetsFilenames []string) ([]updateFile, error) {
	updateFilesInfo := make([]updateFile, 0)
	for _, filename := range assetsFilenames {
		replacementFileInfo, err := vfl.getReplacementFileInfo(filename)
		if err != nil {
			return nil, err
		}
		if replacementFileInfo.PreventFileLoading {
			continue
		}
		tFileName, err := vfl.loadUpdateFileFromSource(filename)
		if err != nil {
			return nil, err
		}
		updateFilesInfo = append(updateFilesInfo, updateFile{
			replacement:           replacementFileInfo,
			tmpFileName:           tFileName,
			curFileRenamed:        false,
			replacementMovedToDir: false,
		})
	}
	return updateFilesInfo, nil
}

func (vfl versionFilesLoader) loadUpdateFileFromSource(filename string) (string, error) {
	reader, err := vfl.version.getAssetContentByFilename(vfl.updateConfig.ApplicationConfig, filename)
	if err != nil {
		return "", err
	}
	return vfl.writeTempFileToDir(reader, filename)
}

func (vfl versionFilesLoader) writeTempFileToDir(readerOrReaderCloser io.Reader, filename string) (_ string, err error) {
	switch readerOrReaderCloser.(type) {
	case io.ReadCloser:
		defer func() {
			rCloseErr := readerOrReaderCloser.(io.ReadCloser).Close()
			if err != nil && rCloseErr != nil {
				err = fmt.Errorf("%v; close file source reader error: %v", err, rCloseErr)
			}
			if err == nil {
				err = rCloseErr
			}
		}()
	}

	var tFile *os.File
	if !vfl.createOrdinaryFiles {
		tFile, err = os.CreateTemp(vfl.destDir, fmt.Sprintf("update-file-*-%s", filename))
		if err != nil {
			return "", err
		}
	} else {
		tFile, err = os.Create(filepath.Join(vfl.destDir, filename))
		if err != nil {
			return "", err
		}
	}

	defer func() {
		tCloseErr := tFile.Close()
		if err != nil && tCloseErr != nil {
			err = fmt.Errorf("%v; close temp file error: %v", err, tCloseErr)
		}
		if err == nil {
			err = tCloseErr
		}
	}()
	_, err = io.Copy(tFile, readerOrReaderCloser)
	return tFile.Name(), err
}
