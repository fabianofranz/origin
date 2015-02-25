package util

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/pkg/term"
	"github.com/golang/glog"
)

func PromptForString(r io.Reader, format string, a ...interface{}) string {
	fmt.Printf(format, a...)
	return readInput(r)
}

func PromptForPasswordString(r io.Reader, format string, a ...interface{}) string {
	if file, ok := r.(*os.File); ok {
		inFd := file.Fd()

		if term.IsTerminal(inFd) {
			oldState, err := term.SaveState(inFd)
			if err != nil {
				glog.V(3).Infof("Unable to save terminal state")
				return PromptForString(r, format, a...)
			}

			fmt.Printf(format, a...)

			term.DisableEcho(inFd, oldState)

			input := readInput(r)

			defer term.RestoreTerminal(inFd, oldState)

			fmt.Printf("\n")

			return input
		}
		glog.V(3).Infof("Stdin is not a terminal")
		return PromptForString(r, format, a...)
	}
	return PromptForString(r, format, a...)
}

func PromptForBool(r io.Reader, format string, a ...interface{}) bool {
	str := PromptForString(r, format, a...)
	switch strings.ToLower(str) {
	case "1", "t", "true", "y", "yes":
		return true
	case "0", "f", "false", "n", "no":
		return false
	}
	fmt.Println("Please enter 'yes' or 'no'.")
	return PromptForBool(r, format, a...)
}

func PromptForStringWithDefault(r io.Reader, def string, format string, a ...interface{}) string {
	s := PromptForString(r, format, a...)
	if len(s) == 0 {
		return def
	}
	return s
}

func readInput(r io.Reader) string {
	if IsTerminal(r) {
		reader := bufio.NewReader(r)
		result, _ := reader.ReadString('\n')
		return strings.TrimSuffix(result, "\n")
	}
	glog.V(3).Infof("Unable to use a TTY")
	var result string
	fmt.Fscan(r, &result)
	return result
}

func IsTerminal(r io.Reader) bool {
	file, ok := r.(*os.File)
	return ok && term.IsTerminal(file.Fd())
}
