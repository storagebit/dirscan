package main

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"syscall"
	"time"
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

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s [directory]\n", os.Args[0])
		os.Exit(1)
	}
	// channel used to stop the spinner
	stop := make(chan bool)

	directory := os.Args[1]

	//defining lists used to hold the filetype information
	var fileTypes []fileType

	//defining lists used to hold the user information
	var users []userInfo

	var totalFilesCount int64
	var totalCapacity int64
	var totalDirectoriesCount int64

	// starting a timer later used to calculate the time it took to scan the directory and to calculate the scan rate
	start := time.Now()

	// starting the spinner
	go spinner(stop, &totalFilesCount, &totalDirectoriesCount, &start)

	// starting the walk of the directory down into the rabbit hole
	filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {

		// if the file is a directory
		if !info.IsDir() {
			//
			totalFilesCount++

			//
			extension := filepath.Ext(path)

			if extension == "" {
				data, err := os.Open(path)
				if err != nil {
					// as we cannot determine if its binary or text we will just call it unknown
					extension = "no_extension_unknown_format"

					// closing the file
					defer data.Close()
					//fmt.Printf("Error opening file %s: %s, skipping\n", path, err)
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
								//
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
					defer data.Close()
				}

			}
			//getting the size of the file
			size := info.Size()

			// adding the size to the total size
			totalCapacity += size

			// getting the owner of the file
			owner, err := user.LookupId(fmt.Sprintf("%d", info.Sys().(*syscall.Stat_t).Uid))
			// if we cannot get the owner we will just use the uid
			if err != nil {
				owner = &user.User{Uid: fmt.Sprintf("%d", info.Sys().(*syscall.Stat_t).Uid)}
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

			// if the file is a directory we will increase the total number of directories
			totalDirectoriesCount++
		}
		return nil
	})

	sort.Sort(bySize(fileTypes))

	sort.Sort(bySizeUser(users))

	stop <- true

	fmt.Printf("\nTotal capacity: %s Total files: %d, Total directories: %d\n", humanReadableSize(totalCapacity), totalFilesCount, totalDirectoriesCount)
	fmt.Printf("Total scanning time: %s\n", time.Since(start).Truncate(time.Millisecond).String())

	for _, fileType := range fileTypes {
		fmt.Printf("%s: %s #of files %d\n", fileType.extension, humanReadableSize(fileType.size), fileType.count)
		for _, userEntry := range fileType.users {
			fmt.Printf("  %s: Capacity %s #of files %d \n", userEntry.name, humanReadableSize(userEntry.size), userEntry.count)
		}
	}

	fmt.Println("")

	for _, userEntry := range users {
		fmt.Printf("%s: Capacity %s #of files %d \n", userEntry.name, humanReadableSize(userEntry.size), userEntry.count)
		for _, ft := range userEntry.filetypes {
			fmt.Printf("  %s: %s #of files %d\n", ft.extension, humanReadableSize(ft.size), ft.count)
		}
	}
}

func humanReadableSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d bytes", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.2f KiB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MiB", float64(size)/(1024*1024))
	} else {
		return fmt.Sprintf("%.2f GiB", float64(size)/(1024*1024*1024))
	}
}

func spinner(stop chan bool, totalFilesCount *int64, totalDirectoriesCount *int64, start *time.Time) {
	frames := []string{"◐", "◓", "◑", "◒"} // Define the frames for the spinner

	for {
		select {
		case <-stop:
			return // Stop the spinner when a value is received on the stop channel
		default:
			for _, frame := range frames {
				duration := time.Since(*start)
				rate := float64(*totalFilesCount) / duration.Seconds()
				fmt.Printf("\r%s Scanning... Files scanned: %d Directories scanned: %d Rate: %.1f files/second", frame, *totalFilesCount, *totalDirectoriesCount, rate) // Print the current frame and loading text on the same line
				time.Sleep(250 * time.Millisecond)                                                                                                                      // Pause for a short time to control the animation speed
			}
		}
	}
}
