package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

// defining variables used to hold the build information when compiling the program
var (
	buildSha1      string // sha1 revision used to build the program
	buildTime      string // when the executable was built
	buildBranch    string // branch used to build the program
	buildOS        string // operating system used to build the program
	buildGoVersion string // go version used to build the program
)

// Struct defining and used to hold information in the context of files and file extensions
type fileType struct {
	extension string
	size      int64
	count     int64
	users     []fileTypeUserInfo
}

// Struct defining and holding the user information about a specific file/file extension
type fileTypeUserInfo struct {
	name  string
	size  int64
	count int64
}

// Struct defining a struct used to hold information in the context of a user
type userInfo struct {
	name      string
	size      int64
	count     int64
	filetypes []userFileType
}

// defining a struct used to hold information in the context of file extension for a user
type userFileType struct {
	extension string
	size      int64
	count     int64
}

// defining type used to sort the file extension information by size descending
type bySize []fileType

// defining type used to sort the user information by size descending
type bySizeUser []userInfo

// Len Swap Less defining functions used to sort the file extension information by size descending
func (a bySize) Len() int           { return len(a) }
func (a bySize) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a bySize) Less(i, j int) bool { return a[i].size > a[j].size }

// Len Swap Less defining functions used to sort the user information by size descending
func (a bySizeUser) Len() int           { return len(a) }
func (a bySizeUser) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a bySizeUser) Less(i, j int) bool { return a[i].size > a[j].size }

// func humanReadableSize is used to convert a size in bytes to a human-readable format
func humanReadableSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	const unit = 1024
	if exp := int64(math.Log(float64(size)) / math.Log(float64(unit))); exp < 7 {
		pre := "KMGTPE"[exp-1]
		return fmt.Sprintf("%.1f %ciB", float64(size)/math.Pow(float64(unit), float64(exp)), pre)
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/math.Pow(float64(unit), 7), 'Z')
}

// func spinner is used to display a spinner on the command line while the program is running
func spinner(stop chan bool, totalFilesCount *int64, totalDirectoriesCount *int64, start *time.Time) {
	// Define the frames for the spinner
	frames := []string{"◐", "◓", "◑", "◒", "\u26A1"}
	for {
		select {
		case <-stop:
			// Stop the spinner when a value is received on the stop channel
			return
		default:
			for _, frame := range frames {
				duration := time.Since(*start)
				rate := float64(*totalFilesCount) / duration.Seconds()
				fmt.Printf("\r%s Scanning... Files scanned: %d Directories scanned: %d Rate: %.0f files/second \033[0K", frame, *totalFilesCount, *totalDirectoriesCount, rate)
				time.Sleep(250 * time.Millisecond)
			}
		}
	}
}

// func to calculate the average file size
func averageFileSize(fileSize int64, fileCount int64) string {
	average := float64(fileSize) / float64(fileCount)
	average = math.Round(average*100) / 100
	return humanReadableSize(int64(average))
}

func main() {

	// define command line arguments
	// -d directory to scan
	// -l enable logging
	// -v verbose logging
	// -i print out the build information
	// -f print out only the file types/extensions information
	// -u print out only the user information
	// -t log file target directory

	// Create a channel to receive signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	loggingEnabled := false
	directoryToScan := "/home"
	verboseEnabled := false
	buildInfo := false
	fileTypesOnly := false
	userInfoOnly := false
	loggingTargetDirectory := "/tmp"

	flag.BoolVar(&loggingEnabled, "l", false, "enable logging")
	flag.StringVar(&loggingTargetDirectory, "t", "/tmp", "log file target directory")
	flag.StringVar(&directoryToScan, "d", "/home", "directory to scan")
	flag.BoolVar(&verboseEnabled, "v", false, "enable verbose and detailed output")
	flag.BoolVar(&buildInfo, "i", false, "print out the build information")
	flag.BoolVar(&fileTypesOnly, "f", false, "print out only the file types/extensions information")
	flag.BoolVar(&userInfoOnly, "u", false, "print out only the user information")

	flag.Parse()

	if buildInfo {
		fmt.Printf("Build date:\t%s\n"+
			"From branch:\t%s\n"+
			"With sha1:\t%s\n"+
			"On:\t\t%s\n"+
			"Using:\t\t%s\n", buildTime, buildBranch, buildSha1, buildOS, buildGoVersion)
		os.Exit(0)
	}

	logFile, err := os.OpenFile(path.Join(loggingTargetDirectory, "dirscan.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to create log logFile: %v", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatalf("Failed to close log logFile: %v", err)
		}
	}(logFile)

	logger := log.New(io.MultiWriter(os.Stdout), "", log.Ltime)
	// If logging is enabled, create a new logger that writes to both the command line and the log logFile.
	if loggingEnabled {
		logger = log.New(io.MultiWriter(os.Stdout, logFile), "", log.Ldate|log.Ltime|log.Lshortfile)
		logger.Printf("Logging enabled. Please find the dirscan.log file located in %s\n", loggingTargetDirectory)
	}

	//start a go routine to listen for signals in the background and to act on them
	go func() {
		sig := <-sigCh
		fmt.Printf("\n")
		logger.Println("Received signal: ", sig)
		logger.Println("Quitting after signal: ", sig)
		logger.Println("Goodbye")
		os.Exit(0)
	}()

	// channel used to stop the spinner
	stop := make(chan bool)

	var directory = directoryToScan
	//logger.Printf("Target directory: %s\n", directory)

	//defining lists used to hold the filetype information
	var fileTypes []fileType

	//defining lists used to hold the user information
	var users []userInfo

	var totalFilesCount int64
	var totalCapacity int64
	var totalDirectoriesCount int64

	// starting a timer later used to calculate the time it took to scan the directory and to calculate the scan rate
	start := time.Now()

	// getting the current user
	currentUser, err := user.Current()
	if err != nil {
		log.Fatal("Error getting current user: ", err)
	}

	logger.Printf("Scanning directory: %s\n", directory)
	logger.Printf("Scanning as user: %s\n", currentUser.Username)

	// starting the spinner
	go spinner(stop, &totalFilesCount, &totalDirectoriesCount, &start)

	// starting the walk of the directory down into the rabbit hole
	err = filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if verboseEnabled {
				logger.Printf("Error walking directory %s: %s, skipping\n", path, err)
			} else {
				return nil
			}
		} else {
			if verboseEnabled {
				logger.Printf("Walking directory %s\n", path)
			}
		}
		// if the is not a directory, process the file
		if !info.IsDir() {
			//
			totalFilesCount++

			// getting the extension of the file
			extension := filepath.Ext(path)

			// if the file has no extension we will try to determine if its binary or plain text/ASCII
			if extension == "" {
				data, err := os.Open(path)
				if err != nil {
					// as we cannot determine if its binary or text we will just call it unknown
					extension = "no_extension_unknown_format"
					if verboseEnabled {
						logger.Printf("Error opening %s: %s, skipping\n", path, err)
					} else {
						return nil
					}
					// closing the file
					defer func(data *os.File) {
						err := data.Close()
						if err != nil {
							if verboseEnabled {
								logger.Printf("Error closing %s: %s, skipping\n", path, err)
							} else {
								return
							}
						}
					}(data)
					if verboseEnabled {
						logger.Printf("Error opening %s: %s, skipping\n", path, err)
					} else {
						return nil
					}
				} else {
					// if we can open the file we will try to determine if its binary or text
					isBinary := false

					// reading the first 10 lines of the file
					fileScanner := bufio.NewScanner(data)
					for i := 0; i < 10 && fileScanner.Scan(); i++ {

						// if we find a character that is not in the range of 32-126 we will assume its binary
						line := fileScanner.Text()
						for _, c := range line {
							if c < 32 || c > 126 {
								isBinary = true
								break
							}
						}
						if isBinary {
							extension = "no_extension_binary"
						} else {
							extension = "no_extension_text"
						}
					}
					// closing the file
					defer func(data *os.File) {
						err := data.Close()
						if err != nil {
							if verboseEnabled {
								logger.Printf("Error closing logFile %s: %s, skipping\n", path, err)
							} else {
								return
							}
						} else {
							if verboseEnabled {
								logger.Printf("Successfully closed logFile %s\n", path)
							}
						}
					}(data)
				}
			}
			//getting the size of the file in bytes
			size := info.Size()

			// adding the size to the total size
			totalCapacity += size

			// getting the owner of the file
			owner, err := user.LookupId(fmt.Sprintf("%d", info.Sys().(*syscall.Stat_t).Uid))
			// if we cannot get the owner we will just use the uid
			if err != nil {
				owner = &user.User{Uid: fmt.Sprintf("%d", info.Sys().(*syscall.Stat_t).Uid)}
				if verboseEnabled {
					logger.Printf("Error getting owner of %s: %s, using uid instead\n", path, err)
				} else {
					return nil
				}
			} else {
				if verboseEnabled {
					logger.Printf("Successfully got owner of %s: %s\n", path, owner.Username)
				}
			}
			// checking if the extension is already in the list
			extensionFound := false

			// looping through the list of file extensions
			for i := range fileTypes {

				// if the extension is already in the list we will add the size to the total size and increase the count
				if fileTypes[i].extension == extension {
					fileTypes[i].size += size
					fileTypes[i].count++
					extensionFound = true

					// checking if the user is already in the list
					userFound := false

					// looping through the list of users
					for j := range fileTypes[i].users {

						// if the user is already in the list we will add the size to the total size and increase the count
						if fileTypes[i].users[j].name == owner.Username {
							fileTypes[i].users[j].size += size
							fileTypes[i].users[j].count++
							userFound = true
							// breaking out of the loop
							break
						}
					}
					// if the user is not in the list we will add it
					if !userFound {
						fileTypes[i].users = append(fileTypes[i].users, fileTypeUserInfo{
							name:  owner.Username,
							size:  size,
							count: 1,
						})
					}
					// breaking out of the loop
					break
				}
			}

			// if the extension is not in the list we will add it
			if !extensionFound {
				fileTypes = append(fileTypes, fileType{
					extension: extension,
					size:      size,
					count:     1,
					users: []fileTypeUserInfo{{
						name:  owner.Username,
						size:  size,
						count: 1,
					}},
				})
			}

			// checking if the user is already in the list
			extensionUserFound := false

			// looping through the list of users
			for i := range users {

				//if the user is already in the list we will add the size to the total size and increase the count
				if users[i].name == owner.Username {
					users[i].size += size
					users[i].count++
					extensionUserFound = true

					// checking if the extension is already in the list
					userFileExtensionFound := false

					// looping through the list of extensions
					for j := range users[i].filetypes {
						// if the extension is already in the list we will add the size to the total size and increase the count
						if users[i].filetypes[j].extension == extension {
							users[i].filetypes[j].size += size
							users[i].filetypes[j].count++
							userFileExtensionFound = true

							// breaking out of the loop
							break
						}
					}
					//checking if the extension is not in the list
					if !userFileExtensionFound {
						// adding the extension to the list
						users[i].filetypes = append(users[i].filetypes, userFileType{
							extension: extension,
							size:      size,
							count:     1,
						})
					}

					// breaking out of the loop
					break
				}
			}
			// if the user is not in the list we will add it
			if !extensionUserFound {

				// adding the user to the list
				users = append(users, userInfo{
					name:  owner.Username,
					size:  size,
					count: 1,
					filetypes: []userFileType{{
						extension: extension,
						size:      size,
						count:     1,
					}},
				})
			}

		} else {

			// if the entry is a directory we will increase the total number of directories
			totalDirectoriesCount++
		}
		return nil
	})

	sort.Sort(bySize(fileTypes))

	sort.Sort(bySizeUser(users))

	stop <- true

	fmt.Println("")
	logger.Printf("Total capacity: %s Total files: %d, Total directories: %d\n", humanReadableSize(totalCapacity), totalFilesCount, totalDirectoriesCount)
	logger.Printf("Total scanning time: %s\n", time.Since(start).Truncate(time.Millisecond).String())

	if !fileTypesOnly {

		logger.Printf("Consumption by user:\n")
		for _, userEntry := range users {
			fmt.Printf("\t%s: Capacity: %s, #of files: %d, average file size: %s \n", userEntry.name, humanReadableSize(userEntry.size), userEntry.count, averageFileSize(userEntry.size, userEntry.count))
			if verboseEnabled {
				for _, ft := range userEntry.filetypes {
					fmt.Printf("\t\t%s: %s #of files: %d average file size: %s\n", ft.extension, humanReadableSize(ft.size), ft.count, averageFileSize(ft.size, ft.count))
				}
			}
		}
	}
	if userInfoOnly {
		os.Exit(0)
	}

	fmt.Println("")
	logger.Printf("Consumption by file type/extension:\n")
	for _, fileTypeEntry := range fileTypes {
		fmt.Printf("\t%s: %s, #of files %d, average filesize: %s\n", fileTypeEntry.extension, humanReadableSize(fileTypeEntry.size), fileTypeEntry.count, averageFileSize(fileTypeEntry.size, fileTypeEntry.count))
		if verboseEnabled {
			for _, userEntry := range fileTypeEntry.users {
				fmt.Printf("\t\t%s: Capacity %s, #of files %d, average filesize: %s \n", userEntry.name, humanReadableSize(userEntry.size), userEntry.count, averageFileSize(userEntry.size, userEntry.count))
			}
		}
	}
}
