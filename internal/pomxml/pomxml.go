package pomxml

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func EnsureModule(path, module string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := string(data)
	if hasModule(content, module) {
		return false, nil
	}
	closeIndex := strings.LastIndex(content, "</modules>")
	if closeIndex < 0 {
		return false, fmt.Errorf("missing </modules> in %s", path)
	}
	newline := "\n"
	if strings.Contains(content, "\r\n") {
		newline = "\r\n"
	}
	indent := inferModuleIndent(content[:closeIndex])
	closeLineStart := strings.LastIndex(content[:closeIndex], "\n")
	if closeLineStart < 0 {
		closeLineStart = closeIndex
	} else {
		closeLineStart++
	}
	insert := fmt.Sprintf("%s<module>%s</module>%s", indent, module, newline)
	updated := content[:closeLineStart] + insert + content[closeLineStart:]
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		return false, err
	}
	return true, nil
}

func hasModule(content, module string) bool {
	pattern := regexp.MustCompile(`<module>\s*` + regexp.QuoteMeta(module) + `\s*</module>`)
	return pattern.FindStringIndex(content) != nil
}

func inferModuleIndent(prefix string) string {
	lastModule := strings.LastIndex(prefix, "<module>")
	if lastModule < 0 {
		return "        "
	}
	lineStart := strings.LastIndex(prefix[:lastModule], "\n")
	if lineStart < 0 {
		lineStart = 0
	} else {
		lineStart++
	}
	linePrefix := prefix[lineStart:lastModule]
	return linePrefix
}
