// +build !windows

package updaterini

func (uR *UpdateResult) deletePrevVersionFiles() (err error) {
	return uR.DeletePreviousVersionFiles(DeleteModPureDelete)
}
