package analysis

type FileType string

const (
	FileTypeProd   FileType = "prod"
	FileTypeTest   FileType = "test"
	FileTypeConfig FileType = "config"
)

type Line struct {
	Number  int
	Content string
}

type FileDiff struct {
	Path       string
	AddedLines []Line
	Raw        string
	Type       FileType
}

type Issue struct {
	File     string
	Line     int
	RuleID   string
	Severity string
	Message  string
}
