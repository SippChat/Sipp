package straw

import (
    "regexp"
    "strings"
)

// Define the formatting map
var formatMap = map[string]string{
    "black":   "\x1b[30m",
    "red":     "\x1b[31m",
    "green":   "\x1b[32m",
    "yellow":  "\x1b[33m",
    "blue":    "\x1b[34m",
    "magenta": "\x1b[35m",
    "cyan":    "\x1b[36m",
    "white":   "\x1b[37m",
    "b":       "\x1b[1m", // Bold
    "i":       "\x1b[3m", // Italic
    "u":       "\x1b[4m", // Underline
    "s":       "\x1b[9m", // Strikethrough
}

var tagRegex = regexp.MustCompile(`<(/?)([a-zA-Z]+)>`)

// Serialize converts input text with formatting tags to a formatted string.
func Serialize(input string) string {
    var builder strings.Builder

    matches := tagRegex.FindAllStringSubmatchIndex(input, -1)
    lastEnd := 0

    for _, match := range matches {
        isClosing := input[match[2]:match[3]] == "/"
        tagName := input[match[4]:match[5]]

        builder.WriteString(input[lastEnd:match[0]])

        if format, ok := formatMap[tagName]; ok {
            if !isClosing {
                builder.WriteString(format)
            } else {
                builder.WriteString("\x1b[0m") // Reset formatting
            }
        }

        lastEnd = match[1]
    }

    builder.WriteString(input[lastEnd:])

    return builder.String()
}

