package utils

import (
    "regexp"
    "strings"
)

func ExtractURLs(text string) []string {
    urlRegex := regexp.MustCompile(`https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`)
    return urlRegex.FindAllString(text, -1)
}

func ContainsURL(text string) bool {
    urlRegex := regexp.MustCompile(`https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`)
    return urlRegex.MatchString(text)
}

func ExtractFirstURL(text string) string {
    urls := ExtractURLs(text)
    if len(urls) > 0 {
        return urls[0]
    }
    return ""
}

func CleanURL(url string) string {
    url = strings.Split(url, "?")[0]
    url = strings.Split(url, "&")[0]
    return strings.TrimSpace(url)
}
