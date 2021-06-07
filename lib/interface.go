package lib

import (
	"log"

	"github.com/cheggaaa/pb"
	"github.com/spf13/cobra"
	"gopkg.in/dixonwille/wmenu.v4"
)

// ask the user to continue or exit. Returns true if continue, false if not.
func WaitForConfirm(cmd *cobra.Command) (start bool) {
	startMenu := wmenu.NewMenu("Continue?")
	startMenu.IsYesNo(0)
	startMenu.LoopOnInvalid()
	startMenu.Action(func(opts []wmenu.Opt) error {
		start = (opts[0].ID == 0)
		return nil
	})
	e := startMenu.Run()
	if e != nil {
		cmd.PrintErrln(e)
		start = false
	}
	cmd.Println()
	return start
}

// set up task, bar interface.
func InitBars(dayCount int, taskCount int, logger *log.Logger) (pool *pb.Pool, dayBar *pb.ProgressBar, taskBar *pb.ProgressBar) {
	dayBar = pb.New(dayCount)
	dayBar.BarStart = "Days Complete: ["
	dayBar.ShowPercent = false
	taskBar = pb.New(taskCount)
	taskBar.BarStart = "Log Parses Complete: ["
	pool, err := pb.StartPool(taskBar, dayBar)
	if err != nil {
		logger.Println("ERROR: Failed to start progess bar.")
	}
	return pool, dayBar, taskBar
}
