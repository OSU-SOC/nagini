package lib

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func TryCreateDir(dir string) (err error) {
	dirInfo, err := os.Stat(dir)
	if os.IsNotExist(err) {
		// directory exists. See if permissions to use and is non-empty.
		baseDirInfo, baseDirErr := os.Stat(filepath.Dir(dir))
		if os.IsNotExist(baseDirErr) {
			err = errors.New(fmt.Sprintf("cannot use parent directory %s: does not exist.", filepath.Dir(dir)))
			return err
		} else if !baseDirInfo.IsDir() {
			err = errors.New(fmt.Sprintf("cannot use parent directory %s: exists but is not a directory.", filepath.Dir(dir)))
			return err
		}

		err = os.Mkdir(dir, 0775)

	} else if !dirInfo.IsDir() {
		// if exists but is not a directory, error out.
		err = errors.New("cannot create output directory: file of same name already exists.")
	} else {
		// directory exists. See if permissions to use and is non-empty.
		f, fErr := os.OpenFile(dir, os.O_RDWR, 0)
		if fErr != nil {
			// failed to write to directory, use this error
			err = errors.New("directory exists. Failed to open for write.")
			return err
		}
		defer f.Close()

		_, dirErr := f.Readdirnames(1)
		if dirErr != io.EOF {
			err = errors.New("cannot use specified directory: directory exists and is non-empty.")
		}
	}

	return err
}
