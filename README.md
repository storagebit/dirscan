# dirscan
Scan a directory tree to report data usage by user and file size and types( file count and capacities)   
Report the average file size for each entry.

## Usage
```
Usage of ./dirscan:
  -d string
        directory to scan (default "/home")
  -f    print out only the file types/extensions information
  -i    print out the build information
  -l    enable logging
  -t string
        log file target directory (default "/var/log")
  -u    print out only the user information
  -v    enable verbose and detailed output
```

## Note
The binary available here on GitHub was build on a fairly new Linux system.
If you see glibc version related errors or messages, you most likely run on a more dated version that my build system, and you need to build it on your system.
