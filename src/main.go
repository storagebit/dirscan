package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

var verbose bool
var debugOn bool

func main() {
	// Parse command line arguments
	var dir string
	var workers int
	flag.StringVar(&dir, "d", "/home", "directory to scan")
	flag.IntVar(&workers, "w", runtime.NumCPU(), "number of workers to use")
	flag.BoolVar(&verbose, "v", false, "verbose output")
	flag.BoolVar(&debugOn, "debug", false, "enable debug output")
	flag.Parse()

	//print if verbose output is enabled
	if verbose {
		fmt.Println("Verbose output is enabled")
	}

	if debugOn {
		fmt.Println("\u261E Debugging is enabled")
		//print all the command line arguments
		fmt.Printf("\u261E Command line executed: %v\n", strings.Join(os.Args, " "))
	}

	// create a channel used to manage signals
	// sigs := make(chan os.Signal, 1)
	sigs := make(chan os.Signal, 1)

	// print the sigs channel debug info
	if debugOn {
		fmt.Printf("\u261E Created sigs channel at: %p\n", sigs)
	}

	// register for SIGINT and SIGTERM signals for proper termination if required
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	//starting the goroutine to handle signals
	go func() {
		//print goroutine debug info
		if debugOn {
			fmt.Printf("\u261E Signal handling routine started.\n")
		}

		select {
		case sig := <-sigs:
			switch sig {
			case syscall.SIGINT:
				fmt.Println("\nKeyboard interrupt CTRL-C received, exiting...")
				os.Exit(0)
			case syscall.SIGTERM:
				fmt.Println("\nSIGTERM signal received, exiting...")
				os.Exit(0)
			}
		}
	}()

	//the number of workers should be at least 2 and cannot be more than the number of CPUs
	if workers < 2 {
		workers = 2
	}
	if workers > runtime.NumCPU() {
		fmt.Printf("Warning: number of workers (%d) is greater than number of CPUs/cores available (%d)!\n", workers, runtime.NumCPU())
		workers = runtime.NumCPU()
	}
	fmt.Printf("Using %d workers.\n", workers)

	// Check if the directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Printf("Error: directory %s does not exist\n", dir)
		return
	}

	// Get the list of files to process
	files := make(chan string)

	//print the files channel debug info
	if debugOn {
		fmt.Printf("\u261E Created files channel at: %p\n", files)
	}

	//create a channel to stop the spinner
	stop := make(chan struct{})

	//print the spinner channel debug info
	if debugOn {
		fmt.Printf("\u261E Created stop channel at: %p\n", stop)
	}

	//counters for processed files, directories and capacity
	var totalFileCount uint64   // counter for processed files
	var totalCapacity uint64    // capacity of all processed files
	var totalDirectories uint64 // counter for processed directories

	go func() {
		//print goroutine info
		if debugOn {
			fmt.Printf("\u261E Directory walking routine started.\n")
		}
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() && path == "/proc" {
				return filepath.SkipDir
			}
			if info.IsDir() && path == "/run" {
				return filepath.SkipDir
			}

			if !info.IsDir() {
				files <- path
			} else {
				if verbose {
					fmt.Printf("Directory:%s\n", path)
				}
				totalDirectories++
			}
			return nil
		})
		if err != nil {
			if verbose || debugOn {
				fmt.Printf("Error: %s\n", err)
			}
			return
		}
		close(files)
	}()

	// Start the workers
	var wg sync.WaitGroup
	wg.Add(workers)

	//get the start time
	startTime := time.Now()

	//print the directory to scan
	fmt.Printf("Scanning directory: %s\n", dir)

	//print the start time
	fmt.Printf("Start time: %s\n", startTime.Format("15:04:05"))

	//start the spinner only if no verbose output is enabled (to avoid spinner and verbose output overlapping) and if debug is disabled
	if !verbose && !debugOn {
		go spinner(stop)
	}

	for i := 0; i < workers; i++ {
		go func() {
			//print goroutine info
			if debugOn {
				fmt.Printf("\u261E Worker routine started.\n")
			}
			for file := range files {
				size, _ := processFile(file)
				totalFileCount++
				totalCapacity += size
			}
			wg.Done()
		}()
	}

	wg.Wait()

	//stop the spinner
	//not needed if verbose output is enabled or if debug is enabled
	if !verbose && !debugOn {
		stop <- struct{}{}
	}

	endTime := time.Now()

	fmt.Printf("\rEnd time: %s                    \n", endTime.Format("15:04:05"))

	//determine the run time
	runTime := endTime.Sub(startTime)
	hours := int(runTime.Hours())
	minutes := int(runTime.Minutes()) % 60
	seconds := int(runTime.Seconds()) % 60

	fmt.Printf("Run/Scan time: %02dh:%02dm:%02ds\n", hours, minutes, seconds)

	fmt.Printf("Processed %d files in %d directories and capacity of %s. The average file size is: %s\n",
		totalFileCount, totalDirectories, formatSize(totalCapacity), formatSize(totalCapacity/totalFileCount))
}

func processFile(file string) (uint64, error) {

	//check if the file is a symlink
	if fileInfo, err := os.Lstat(file); err == nil {
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			if verbose || debugOn {
				fmt.Printf("File:\t\t%s: is a symlink, skipping...\n", file)
			}
			return 0, nil
		}
	}

	// stat the file
	fileInfo, err := os.Stat(file)

	if err != nil {
		if os.IsNotExist(err) {
			if verbose || debugOn {
				fmt.Printf("File:\t\t%s: does not exist, skipping...\n", file)
			}
			return 0, nil
		}
		if verbose || debugOn {
			fmt.Printf("Error: %s\n", err)
		}
		return 0, err
	}
	// print the file info if verbose output is enabled or if debug is enabled
	if verbose || debugOn {
		fmt.Printf("File:\t\t%s: %d bytes owner: %d\n", file, fileInfo.Size(), fileInfo.Sys().(*syscall.Stat_t).Uid)
	}
	return uint64(fileInfo.Size()), nil
}

func formatSize(size uint64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

func spinner(stop chan struct{}) {
	// Set the spinner characters
	chars := []string{"✶", "✸", "✹", "✺", "✹", "✸", "✷"}
	i := 0
	for {
		select {
		case <-stop:
			fmt.Print("")
			return
		default:
			fmt.Printf("\r%s Scanning, please wait.", chars[i%len(chars)])
			i++
			time.Sleep(100 * time.Millisecond)
		}
	}
}
