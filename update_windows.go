//go:build windows
// +build windows

package updaterini

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

/*
	for Windows OS executable should be stopped! USE ONLY WITH cur executable file stops functions
*/
func (uR *UpdateResult) deletePrevVersionFiles() (err error) {
	var errFiles []string
	for _, file := range uR.updateFilesInfo {
		if !file.curFileRenamed {
			continue
		}
		fPath := filepath.Join(uR.updateDir, file.replacement.FileName+oldVersionReplacedFilesExtension)
		err = os.Remove(fPath)
		if err != nil {
			errFiles = append(errFiles, fPath)
		}
	}
	if len(errFiles) == 0 {
		return nil
	}

	var sI syscall.StartupInfo
	var pI syscall.ProcessInformation
	argv, tErr := syscall.UTF16PtrFromString(fmt.Sprintf(`%s\system32\cmd.exe /C PING -n 11 127.0.0.1>nul & DEL "%s"`,
		os.Getenv("windir"), strings.Join(errFiles, `", "`))) // 11 equal 10 seconds, 31 equal 30 seconds etc.
	if tErr != nil {
		return tErr
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

	return err
}

func (rF *updateFile) fillFileOwnerInfo(_ os.FileInfo) {
	rF.curFileOwner = -1
	rF.curFileGroup = -1
}
