package updaterini

import (
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

type file struct {
	relPath    string
	shouldStay bool
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

	// create test dir
	resultFiles := make(map[string]bool)
	for _, tFile := range files {
		fPath := filepath.Join(tempDir, tFile.relPath)
		if tFile.shouldStay {
			resultFiles[fPath] = false
		}
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

	// check results
	err = filepath.WalkDir(tempDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Fatalf("dir walk err %s", err)
		}
		if d.IsDir() {
			return nil
		}
		if _, ok := resultFiles[path]; !ok {
			t.Errorf("file should be deleted filepath: %s", path)
		}
		resultFiles[path] = true
		return nil
	})
	if err != nil {
		t.Fatalf("dir walk err %s", err)
	}

	for fPath, fileResultStatus := range resultFiles {
		if !fileResultStatus {
			t.Errorf("file is deleted, but should't : %s", fPath)
		}
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

//TODO TEST ROLLBACK

//TODO TEST UNSAFEROLLBACK
