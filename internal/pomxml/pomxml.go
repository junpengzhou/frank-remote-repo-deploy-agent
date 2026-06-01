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
		// 已存在就保持文件不变，避免重复 module 影响 Maven 聚合构建。
		return false, nil
	}
	closeIndex := strings.LastIndex(content, "</modules>")
	if closeIndex < 0 {
		return false, fmt.Errorf("missing </modules> in %s", path)
	}
	newline := "\n"
	if strings.Contains(content, "\r\n") {
		// 保留原文件换行风格，减少不必要的 diff。
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
	// 允许 module 标签内部有空白，兼容手工编辑过的 pom.xml。
	pattern := regexp.MustCompile(`<module>\s*` + regexp.QuoteMeta(module) + `\s*</module>`)
	return pattern.FindStringIndex(content) != nil
}

func inferModuleIndent(prefix string) string {
	// 新 module 使用最后一个已有 module 的缩进，保持模板文件风格一致。
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
