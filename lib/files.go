package lib

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
)

func TryCreateDir(dir string, empty bool) (err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return errors.New(fmt.Sprintf("failed to resolve path %s.", dir))
	}
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

		err = os.Mkdir(dir, 0665)

	} else if !dirInfo.IsDir() {
		// if exists but is not a directory, error out.
		err = errors.New("cannot create output directory: file of same name already exists, and is not a directory.")
	} else {
		// directory exists. See if permissions to use and is non-empty.
		f, fErr := os.OpenFile(dir, os.O_RDWR, 0)
		if fErr != nil {
			// failed to write to directory, use this error
			return err
		}
		defer f.Close()

		if empty {
			_, dirErr := f.Readdirnames(1)
			if dirErr != io.EOF {
				err = errors.New("cannot use specified directory: directory exists and is non-empty.")
			}
		}
	}

	return err
}

// takes a list of files, sorts them and concats them into a single file. if deleteInputAfterRead, also deletes the input after use.
func ConcatFiles(logger *log.Logger, inputFiles []string, outputFile string, deleteInputAfterRead bool) (e error) {
	// try to create outputFile
	outFd, fcErr := os.Create(outputFile)
	if fcErr != nil {
		return fcErr
	}

	// no error. Sort alphabetically (therefore in time order)
	sort.Strings(inputFiles)

	// for every input file, concat together.
	for _, inputFile := range inputFiles {
		logger.Printf("Concatting %s into %s\n", inputFile, outputFile)
		tempFd, err := os.Open(inputFile)
		if err != nil {
			logger.Printf("ERROR: could not read file '%s': %s\n", inputFile, err)
			continue
		}

		// read temp file and write to final output file
		scanner := bufio.NewScanner(tempFd)
		for scanner.Scan() {
			outFd.WriteString(scanner.Text() + "\n")
		}

		// close temp file as we no longer need it.
		tempFd.Close()

		// if delete flag is set to true, delete the input file.
		if deleteInputAfterRead {
			err = os.Remove(inputFile)
			if err != nil {
				logger.Printf("ERROR: could not remove temp file '%s': %s\n", fcErr, err)
			}
		}
	}

	return outFd.Close()
}
