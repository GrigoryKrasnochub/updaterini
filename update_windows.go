// +build windows

package updaterini

import (
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
		fPath := filepath.Join(uR.updateDir, file.fileName+oldVersionReplacedFilesExtension)
		err = os.Remove(fPath)
		if err != nil {
			errFiles = append(errFiles, fPath)
		}
	}
	var sI syscall.StartupInfo
	var pI syscall.ProcessInformation
	argv, tErr := syscall.UTF16PtrFromString(os.Getenv("windir") + "\\system32\\cmd.exe timeout /T 10 /C del " + strings.Join(errFiles, ", "))
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
