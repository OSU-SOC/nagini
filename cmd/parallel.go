/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	lib "github.com/OSU-SOC/nagini/lib"
)

// args
var threads uint     // number of threads to run
var timeRange string // string format of time range to go over

// calculated start time and end time values
var startTime time.Time
var endTime time.Time

// parallelCmd represents the parallel command
var parallelCmd = &cobra.Command{
	Use:   "parallel",
	Short: "Legacy parallel for backwards compatibility with old parallel.py.",
	Long: `Legacy parallel for backwards compatibility with old parallel.py.

Example:
	nagini parallel 8 my_script.py 

where my_script.py has the following required syntax:
	./my_script.py [input file] [output file]
`,
	Args: cobra.MinimumNArgs(1), // 1 argument: script to run
	Run: func(cmd *cobra.Command, args []string) {
		var dateStrings = strings.Split(timeRange, "-")
		startTime, startErr := time.Parse(lib.TimeFormatShort, dateStrings[0])
		endTime, endErr := time.Parse(lib.TimeFormatShort, dateStrings[1])

		if startErr != nil || endErr != nil {
			panic("Provided dates malformed. Please provide dates in the following format: YYYY/MM/DD:HH-YYYY/MM/DD:HH")
		}
		fmt.Printf("will run through dates %s through %s", startTime, endTime)
	},
}

func init() {
	rootCmd.AddCommand(parallelCmd)

	// Add flags
	// threads
	parallelCmd.PersistentFlags().UintVarP(&threads, "threads", "t", 1, "Number of threads to run in parallel")
	// time range to parse
	parallelCmd.PersistentFlags().StringVarP(
		&timeRange, "timeRange", "r",
		fmt.Sprintf( // write range of last 24 hours
			"%s-%s",
			time.Now().AddDate(0, 0, -1).Format(lib.TimeFormatShort), // yesterday at this time
			time.Now().Format(lib.TimeFormatShort)),                  // right now
		"time-range (local time). unspecified: last 24 hours. Format: YYYY/MM/DD:HH-YYYY/MM/DD:HH",
	)
}
