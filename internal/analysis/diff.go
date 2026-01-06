package analysis

import (
	"fmt"
	"strings"
)

func ParseUnifiedDiff(diff string) ([]FileDiff, error) {
	if diff == "" {
		return nil, nil
	}

	var files []FileDiff
	var current *FileDiff
	var lineBuffer []string
	newLine := 0

	flush := func() {
		if current == nil {
			return
		}
		current.Raw = strings.Join(lineBuffer, "\n")
		files = append(files, *current)
		current = nil
		lineBuffer = nil
		newLine = 0
	}

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			path, err := parseDiffPath(line)
			if err != nil {
				return nil, err
			}
			fileType := ClassifyPath(path)
			current = &FileDiff{Path: path, Type: fileType}
		}

		if current == nil {
			continue
		}

		lineBuffer = append(lineBuffer, line)

		if strings.HasPrefix(line, "@@") {
			var err error
			newLine, err = parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			continue
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			current.AddedLines = append(current.AddedLines, Line{
				Number:  newLine,
				Content: strings.TrimPrefix(line, "+"),
			})
			newLine++
			continue
		}

		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			continue
		}

		if newLine > 0 && !strings.HasPrefix(line, "\\") {
			newLine++
		}
	}

	flush()
	return files, nil
}

func parseDiffPath(line string) (string, error) {
	parts := strings.Split(line, " ")
	if len(parts) < 4 {
		return "", fmt.Errorf("invalid diff header: %s", line)
	}
	path := strings.TrimPrefix(parts[3], "b/")
	if path == "" {
		return "", fmt.Errorf("invalid diff path: %s", line)
	}
	return path, nil
}

func parseHunkHeader(line string) (int, error) {
	parts := strings.Split(line, " ")
	for _, part := range parts {
		if strings.HasPrefix(part, "+") {
			rangePart := strings.TrimPrefix(part, "+")
			rangePart = strings.TrimPrefix(rangePart, "-")
			values := strings.Split(rangePart, ",")
			if len(values) == 0 {
				break
			}
			start, err := parseInt(values[0])
			if err != nil {
				return 0, err
			}
			return start, nil
		}
	}
	return 0, fmt.Errorf("invalid hunk header: %s", line)
}

func parseInt(value string) (int, error) {
	num := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid integer: %s", value)
		}
		num = num*10 + int(r-'0')
	}
	return num, nil
}
