package lib

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/cheggaaa/pb"
	"github.com/spf13/cobra"
)

// tries to create a directory at the given path.
// the parent directory must already exist.
// if the directory already exists, will check to make sure write permissions
// - additionally, if the empty flag is set, then it will enforce that the
//   directory is empty.
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

		err = os.Mkdir(dir, 0775)

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

// Waits until the given sync group is done. When it finishes, concats all files together of that particular date, and then lets the global sync group know it has finished.
func ConcatFilesParallelByDate(logType string, inputFiles []string, outputFile, outputDir string, logger *log.Logger, curDate time.Time, wgDate *sync.WaitGroup, wgAll *sync.WaitGroup, bar *pb.ProgressBar) {
	// Wait for all log files for this date to finish.
	wgDate.Wait()
	defer wgAll.Done()
	defer bar.Increment()

	logger.Printf("All logs for %s finished. Concatinating into '%s'\n", curDate.Format(TimeFormatDate), outputFile)

	// keep track of concat failures to alert the program.
	failure := false

	// if no input files, ignore.
	if len(inputFiles) == 0 {
		logger.Printf("WARN: No matches for date %s. Skipping.\n", curDate.Format(TimeFormatDate))
	} else {
		e := ConcatFiles(logger, inputFiles, outputFile, true, false)
		if e != nil {
			logger.Println("ERROR: ", e)
			failure = true
		}
	}

	// print whether or not we failed to concat the files together.
	if failure {
		logger.Printf("FAIL: %s\n", curDate.Format(TimeFormatDate))
	} else {
		logger.Printf("SUCCESS: %s\n", curDate.Format(TimeFormatDate))
	}
}

// takes a list of files and writes them to STDOUT
func ConcatToStdout(logger *log.Logger, inputFiles []string, deleteInputAfterRead bool, ignoreMissing bool) (e error) {
	return concatFilesToFd(logger, inputFiles, os.Stdout, deleteInputAfterRead, ignoreMissing)
}

// takes a list of files, sorts them and concats them into a single file. if deleteInputAfterRead, also deletes the input after use.
func ConcatFiles(logger *log.Logger, inputFiles []string, outputFile string, deleteInputAfterRead bool, ignoreMissing bool) (e error) {
	// try to create outputFile
	outFd, fcErr := os.Create(outputFile)
	if fcErr != nil {
		return fcErr
	}
	return concatFilesToFd(logger, inputFiles, outFd, deleteInputAfterRead, ignoreMissing)
}

// takes the given os.File and the list of inputFiles, and writes to it in-order.
// used by Concat exported functions.
func concatFilesToFd(logger *log.Logger, inputFiles []string, outFd *os.File, deleteInputAfterRead bool, ignoreMissing bool) (e error) {

	// no error. Sort alphabetically (therefore in time order)
	sort.Strings(inputFiles)

	// for every input file, concat together.
	for _, inputFile := range inputFiles {
		tempFd, err := os.Open(inputFile)
		if err != nil {
			if !ignoreMissing {
				logger.Printf("ERROR: could not read file '%s': %s\n", inputFile, err)
			}
			continue
		}
		logger.Printf("Concatting %s\n", inputFile)

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
				logger.Printf("ERROR: could not remove temp file '%s': %s\n", inputFile, err)
			}
		}
	}

	return outFd.Close()
}

// takes a log type, time range, zeek log directory, thread information, and output directory info.
// it then parses logs based on the logHandler and then outputs the files to the given directory, all parallelized.
func ParseLogs(cmd *cobra.Command, logHandler func(string, string, time.Time, *sync.WaitGroup, *pb.ProgressBar), logger *log.Logger, startTime time.Time, endTime time.Time, logType string, resolvedLogDir string, resolvedOutDir string, threads int, singleFile bool, writeStdout bool) {
	var taskCount = 0

	// create the output directory.
	e := TryCreateDir(resolvedOutDir, true)
	if e != nil {
		cmd.PrintErrln(e)
	} else {
		logger.Printf("created dir %s\n", resolvedOutDir)
	}

	var outputFiles []string

	// set parallel routine thread limit
	runtime.GOMAXPROCS(threads)

	// time iterators
	curDate := startTime.Truncate(24 * time.Hour) // start at this date, at 00:00:00
	curTime := startTime                          // start at this hour

	// progress bars init
	dayCount := int(endTime.Round(time.Hour*24).Sub(startTime.Truncate(time.Hour*24)).Hours() / 24.0) // calculate total number of days
	barPool, dayBar, taskBar := InitBars(dayCount, taskCount, logger)

	// holds wait interface for all routines to finish.
	var wgAll sync.WaitGroup

	// for each date
	for curDate.Before(endTime) || curDate.Equal(endTime) {
		// holds wait interface for all routines of this particular day.
		var wgDate sync.WaitGroup
		var tempFiles []string
		// for each hour of that date, excluding the last date where we may end early.
		for curTime.Before(curDate.AddDate(0, 0, 1)) && (curTime.Before(endTime) || curTime.Equal(endTime)) {
			// find all input files that match this hour
			inputFileGlob := fmt.Sprintf("%s/%04d-%02d-%02d/%s.%02d*", resolvedLogDir, curTime.Year(), curTime.Month(), curTime.Day(), logType, curTime.Hour())
			logFileMatches, e := filepath.Glob(inputFileGlob)
			if e != nil {
				logger.Printf("ERROR (%s): %s\n", curTime.Format(TimeFormatHuman), e)
				continue
			}
			taskCount += len(logFileMatches) // set total number of found log files, plus one for the concatenation step.
			taskBar.SetTotal(taskCount)      // set new total on bar to include found log files
			taskBar.Update()

			// for every found log file, run the script.
			for _, logFile := range logFileMatches {
				outputFileTemp := filepath.Join(
					resolvedOutDir,
					curTime.Format(TimeFormatDateNum)+filepath.Base(logFile)+".json",
				)
				tempFiles = append(tempFiles, outputFileTemp)

				// handle logs based on given input of a log file and a place to output the data,
				// also given the current hour we are looking at, a sync group to sync on, and a
				// task bar to update.
				logHandler(logFile, outputFileTemp, curTime, &wgDate, taskBar)
			}
			curTime = curTime.Add(time.Hour)
		}

		// wait for all date's to finish each log and then for them to concat into a single file.
		wgAll.Add(1)

		// determine output file and concat all temp files by date to it.
		outputFile := filepath.Join(
			resolvedOutDir,
			fmt.Sprintf("%s-%04d-%02d-%02d.json", logType, curDate.Year(), curDate.Month(), curDate.Day()),
		)
		outputFiles = append(outputFiles, outputFile)
		go ConcatFilesParallelByDate(logType, tempFiles, outputFile, resolvedOutDir, logger, curDate, &wgDate, &wgAll, dayBar)

		// iterate to next date
		curDate = curDate.AddDate(0, 0, 1)
	}

	// wait for each day's go routine to finish. When done, exit!
	logger.Println("All routines queued. Waiting for them to finish.")

	wgAll.Wait()

	// if we want to write to stdout, concat output directory, write to std, then delete output directory.
	if writeStdout {
		// read all output to stdout
		e = ConcatToStdout(logger, outputFiles, true, true)
		if e != nil {
			cmd.PrintErrln(e)
		}

		// delete output dir, if possible.
		e = os.Remove(resolvedOutDir)
		if e != nil {
			logger.Printf("ERROR: could not remove temp directory '%s': %s\n", resolvedOutDir, e)
		}
	} else if singleFile {
		// not stdout and singleFile flag set, so we should write to a single file.
		cmd.Printf("Concat flag set. Concatting all output into a single %s.json file.\n", logType)
		e = ConcatFiles(logger, outputFiles, filepath.Join(resolvedOutDir, fmt.Sprintf("%s.json", logType)), true, true)
		if e != nil {
			cmd.PrintErrln(e)
		}
	}

	barPool.Stop()
}
