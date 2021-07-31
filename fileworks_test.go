package updaterini

import (
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

type file struct {
	relPath              string
	shouldStay           bool
	contentBeforeReplace *string
	contentAfterReplace  *string
}

func testFilesFunction(files []file, fileWorksFunction func(appDir string) error, t *testing.T) {
	tempDir, err := ioutil.TempDir("", "tft-*")
	if err != nil {
		t.Fatalf("create temp dir err %s", err)
	}
	defer func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			t.Errorf("delete temp dir err %s", err)
		}
	}()

	// create test dirs
	resultFiles := make(map[string]file)
	for _, tFile := range files {
		fPath := filepath.Join(tempDir, tFile.relPath)
		resultFiles[fPath] = tFile
		dirPath, _ := filepath.Split(tFile.relPath)
		if dirPath != "" {
			err = os.MkdirAll(filepath.Join(tempDir, dirPath), os.ModeDir)
			if err != nil {
				t.Fatalf("create subdirs err %s", err)
			}
		}
		nFile, err := os.Create(fPath)
		if err != nil {
			t.Fatalf("create file err %s", err)
		}
		if tFile.contentBeforeReplace != nil {
			_, err = nFile.WriteString(*tFile.contentBeforeReplace)
			if err != nil {
				t.Fatalf("file write err %s", err)
			}
		}
		err = nFile.Close()
		if err != nil {
			t.Fatalf("close file err %s", err)
		}
	}

	// do something with dir files
	err = fileWorksFunction(tempDir)
	if err != nil {
		t.Fatalf("some fileWorks func err %s", err)
	}

	// check expected files
	for fPath, rFileInfo := range resultFiles {
		if _, err := os.Stat(fPath); os.IsNotExist(err) {
			if rFileInfo.shouldStay {
				t.Errorf("file shoudn`t be deleted. filepath: %s", fPath)
			}
			continue
		}

		fData, err := os.ReadFile(fPath)
		if err != nil {
			t.Fatalf("file read err %s", err)
		}
		if (rFileInfo.contentAfterReplace == nil && len(fData) != 0) || (rFileInfo.contentAfterReplace != nil && string(fData) != *rFileInfo.contentAfterReplace) {
			expectedContent := "nil"
			if rFileInfo.contentAfterReplace != nil {
				expectedContent = *rFileInfo.contentAfterReplace
			}
			t.Errorf("file content is incorect. filepath: %s; expected content: %s; fact content: %s", fPath, expectedContent, fData)
		}
	}

	// check unexpected files
	err = filepath.WalkDir(tempDir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		if _, ok := resultFiles[path]; !ok {
			t.Errorf("undefined file err, posible test file creation err. filepath: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("dir walk err %s", err)
	}
}

func TestUnsafeFileDelete(t *testing.T) {
	files := []file{
		{relPath: "test.exe", shouldStay: true},
		{relPath: "test.exe.old", shouldStay: false},
		{relPath: "test.dll", shouldStay: true},
	}
	testFilesFunction(files, UnsafeDeletePreviousVersionFiles, t)
}

func TestUnsafeFileDelete1(t *testing.T) {
	files := []file{
		{relPath: "test.exe", shouldStay: true},
		{relPath: filepath.Join("builds", "tests", "test.exe"), shouldStay: true},
		{relPath: filepath.Join("builds", "tests", "test.dll.old"), shouldStay: false},
		{relPath: filepath.Join("builds", "tests", "test.dll"), shouldStay: true},
		{relPath: "test.exe.old", shouldStay: false},
		{relPath: "test.dll", shouldStay: true},
	}
	testFilesFunction(files, UnsafeDeletePreviousVersionFiles, t)
}

func TestUnsafeFileDelete2(t *testing.T) {
	files := []file{
		{relPath: "test.exe", shouldStay: true},
		{relPath: filepath.Join("builds", "tests", "test.dll.old"), shouldStay: true},
		{relPath: filepath.Join("builds", "test.dll.old"), shouldStay: true},
		{relPath: "test.dll", shouldStay: true},
	}
	testFilesFunction(files, UnsafeDeletePreviousVersionFiles, t)
}

func TestUnsafeRollbackUpdate(t *testing.T) {
	exeFileContent := "exe test content"
	files := []file{
		{relPath: "test.exe", shouldStay: true, contentAfterReplace: &exeFileContent},
		{relPath: "test.exe.old", shouldStay: false, contentBeforeReplace: &exeFileContent},
		{relPath: "test.dll", shouldStay: true},
	}
	testFilesFunction(files, func(appDir string) error {
		rR, err := UnsafeRollbackUpdate(appDir)
		if err != nil {
			return err
		}
		return rR.DeleteLoadedVersionFiles(DeleteModPureDelete)
	}, t)
}

func TestUnsafeRollbackUpdate1(t *testing.T) {
	exeFileContent := "exe test content"
	exeFileContentOldExe := "exe test content old exe"
	dllFileContent := "dll test content"
	files := []file{
		{relPath: "test.exe", shouldStay: true, contentAfterReplace: &exeFileContent},
		{relPath: "test.exe.old", shouldStay: false, contentBeforeReplace: &exeFileContent},
		{relPath: "build/test.exe.old", shouldStay: false, contentAfterReplace: &exeFileContentOldExe, contentBeforeReplace: &exeFileContentOldExe},
		{relPath: "test.dll", shouldStay: true, contentAfterReplace: &dllFileContent},
		{relPath: "test.dll.old", shouldStay: false, contentBeforeReplace: &dllFileContent},
	}
	testFilesFunction(files, func(appDir string) error {
		rR, err := UnsafeRollbackUpdate(appDir)
		if err != nil {
			return err
		}
		return rR.DeleteLoadedVersionFiles(DeleteModPureDelete)
	}, t)
}
