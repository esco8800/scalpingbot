package tools

import "log"

func LogErrorf(format string, args ...interface{}) {
    red := "\033[31m"
    reset := "\033[0m"
    log.Printf(red+format+reset, args...)
}
