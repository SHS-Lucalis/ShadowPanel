package daemon

import "time"

type CommandResult struct {
	Output   string
	ExitCode int
}

type NodeStatus struct {
	Uptime        time.Duration
	Version       string
	BuildDate     string
	WorkingTasks  int
	WaitingTasks  int
	OnlineServers int
}

type NodeVersion struct {
	Version   string
	BuildDate string
}

// FileInfo represents basic information about a file or directory.
type FileInfo struct {
	Name         string
	Size         uint64
	TimeModified uint64
	Type         FileType
	Perm         uint32
}

// FileDetails represents detailed information about a file or directory.
type FileDetails struct {
	Name             string
	Mime             string
	Size             uint64
	ModificationTime uint64
	AccessTime       uint64
	CreateTime       uint64
	Perm             uint32
	Type             FileType
}

type FileType uint8

const (
	FileTypeUnknown     FileType = 0
	FileTypeDir         FileType = 1
	FileTypeFile        FileType = 2
	FileTypeDevice      FileType = 3
	FileTypeBlockDevice FileType = 4
	FileTypeNamedPipe   FileType = 5
	FileTypeSymlink     FileType = 6
	FileTypeSocket      FileType = 7
)
