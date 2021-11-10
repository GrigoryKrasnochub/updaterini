//go:build !windows
// +build !windows

package updaterini

import (
	"os"
	"syscall"
)

func (uR *UpdateResult) deletePrevVersionFiles() (err error) {
	return uR.DeletePreviousVersionFiles(DeleteModPureDelete)
}

func (rF *updateFile) fillFileOwnerInfo(fInfo os.FileInfo) {
	if fileSysInfo := fInfo.Sys(); fileSysInfo != nil {
		unixFSI := fileSysInfo.(*syscall.Stat_t)
		rF.curFileOwner = int(unixFSI.Uid)
		rF.curFileGroup = int(unixFSI.Gid)
	} else {
		rF.curFileOwner = -1
		rF.curFileGroup = -1
	}
}
