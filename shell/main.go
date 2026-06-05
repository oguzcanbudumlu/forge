package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// Ensures gofmt doesn't remove the "fmt" import in stage 1 (feel free to remove this!)
var _ = fmt.Print

func parseInput(input string) []string {
	var args []string
	var currentArg strings.Builder
	inSingleQuotes := false
	inDoubleQuotes := false
	isEscaped := false
	hasChar := false

	runes := []rune(input)
	// for _, r := range input {
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if isEscaped {
			currentArg.WriteRune(r)
			hasChar = true
			isEscaped = false
			continue
		}

		if inDoubleQuotes && r == '\\' {
			if i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\\') {
				currentArg.WriteRune(runes[i+1])
				hasChar = true
				i++
			} else {
				currentArg.WriteRune(r)
				hasChar = true
			}
			continue
		}

		if r == '\\' && !inSingleQuotes && !inDoubleQuotes {
			isEscaped = true
			continue
		}

		if r == '\'' && !inDoubleQuotes {
			inSingleQuotes = !inSingleQuotes
			hasChar = true
			continue
		}

		if r == '"' && !inSingleQuotes {
			inDoubleQuotes = !inDoubleQuotes
			hasChar = true
			continue
		}

		if r == ' ' && !inSingleQuotes && !inDoubleQuotes {
			if currentArg.Len() > 0 || hasChar {
				args = append(args, currentArg.String())
				currentArg.Reset()
				hasChar = false
			}
			continue
		}

		currentArg.WriteRune(r)
		hasChar = true
	}

	if currentArg.Len() > 0 || hasChar {
		args = append(args, currentArg.String())
	}

	return args
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	builtins := map[string]bool{
		"exit": true,
		"echo": true,
		"type": true,
		"pwd":  true,
		"cd":   true,
	}

	autocompleteBuiltins := make([]string, 0, len(builtins))
	for k := range builtins {
		autocompleteBuiltins = append(autocompleteBuiltins, k)
	}
	sort.Strings(autocompleteBuiltins)

	for {
		fmt.Print("$ ")

		var currentInput strings.Builder
		// input, err := reader.ReadString('\n')
		// if err != nil {
		// 	fmt.Println(os.Stderr, "Error reading input: ", err)
		// 	os.Exit(2)
		// }

		for {
			r, _, err := reader.ReadRune()
			if err != nil {
				os.Exit(0)
			}

			if r == '\t' {
				currentStr := currentInput.String()
				matched := ""
				matchCount := 0

				for _, b := range autocompleteBuiltins {
					if strings.HasPrefix(b, currentStr) && currentStr != "" {
						matched = b
						matchCount++
					}
				}

				if matchCount == 1 {
					remainder := strings.TrimPrefix(matched, currentStr)
					completion := remainder + " "
					fmt.Print(completion)

					os.Stdout.Sync()

					currentInput.WriteString(completion)
				} else {
					fmt.Println("\a")
					os.Stdout.Sync()
				}
				continue
			}

			if r == '\n' || r == '\r' {
				fmt.Print("\n")
				break
			}

			fmt.Print(string(r))
			currentInput.WriteRune(r)
		}

		input := strings.TrimSpace(currentInput.String())

		if input == "" {
			continue
		}

		// args := strings.Fields(input)

		args := parseInput(input)
		if len(args) == 0 {
			continue
		}

		var redirectOutPath string
		var redirectErrPath string
		var isAppendOut bool
		var isAppendErr bool
		var cleanArgs []string

		for i := 0; i < len(args); i++ {
			if (args[i] == ">" || args[i] == "1>") && i+1 < len(args) {
				redirectOutPath = args[i+1]
				isAppendOut = false
				i++
			} else if (args[i] == ">>" || args[i] == "1>>") && i+1 < len(args) {
				redirectOutPath = args[i+1]
				isAppendOut = true
				i++
			} else if args[i] == "2>" && i+1 < len(args) {
				redirectErrPath = args[i+1]
				isAppendErr = false
				i++
			} else if args[i] == "2>>" && i+1 < len(args) {
				redirectErrPath = args[i+1]
				isAppendErr = true
				i++
			} else {
				cleanArgs = append(cleanArgs, args[i])
			}
		}

		args = cleanArgs
		if len(args) == 0 {
			continue
		}

		command := args[0]

		var outFile *os.File
		var errFile *os.File
		var fileErr error

		if redirectOutPath != "" {
			flags := os.O_WRONLY | os.O_CREATE
			if isAppendOut {
				flags |= os.O_APPEND
			} else {
				flags |= os.O_TRUNC
			}

			outFile, fileErr = os.OpenFile(redirectOutPath, flags, 0666)
			if fileErr != nil {
				fmt.Fprintln(os.Stderr, "Error opening stdout file:", fileErr)
				continue
			}
		}

		if redirectErrPath != "" {
			flags := os.O_WRONLY | os.O_CREATE
			if isAppendErr {
				flags |= os.O_APPEND
			} else {
				flags |= os.O_TRUNC
			}

			errFile, fileErr = os.OpenFile(redirectErrPath, flags, 0666)
			if fileErr != nil {
				fmt.Fprintln(os.Stderr, "Error opening stderr file:", fileErr)
				continue
			}
		}

		closeFiles := func() {
			if outFile != nil {
				outFile.Close()
			}
			if errFile != nil {
				errFile.Close()
			}
		}

		printOutput := func(output string) {
			if outFile != nil {
				fmt.Fprintln(outFile, output)
			} else {
				fmt.Println(output)
			}
		}

		printError := func(errorMessage string) {
			if errFile != nil {
				fmt.Fprintln(errFile, errorMessage)
			} else {
				fmt.Fprintln(os.Stderr, errorMessage)
			}
		}

		if command == "exit" {
			closeFiles()
			os.Exit(0)
		}

		if command == "echo" {
			output := strings.Join(args[1:], " ")
			printOutput(output)
			closeFiles()
			continue
		}

		if command == "type" {
			if len(args) < 2 {
				closeFiles()
				continue
			}

			targetCommand := args[1]

			if builtins[targetCommand] {
				fmt.Printf("%s is a shell builtin\n", targetCommand)
				continue
			}

			var result string
			fullPath, err := exec.LookPath(targetCommand)
			if err == nil {
				result = fmt.Sprintf("%s is %s", targetCommand, fullPath)
			} else {
				result = fmt.Sprintf("%s: not found", targetCommand)
			}

			printOutput(result)
			if outFile != nil {
				outFile.Close()
			}
			continue
		}

		if command == "pwd" {
			dir, err := os.Getwd()
			if err != nil {
				printError(fmt.Sprintf("Error getting current directory: %v", err))
				closeFiles()
				continue
			}
			printOutput(dir)
			if outFile != nil {
				outFile.Close()
			}
			continue
		}

		if command == "cd" {
			if len(args) < 2 {
				closeFiles()
				continue
			}

			targetDir := args[1]

			if targetDir == "~" {
				homeDir := os.Getenv("HOME")
				if homeDir != "" {
					targetDir = homeDir
				}
			}
			err := os.Chdir(targetDir)

			if err != nil {
				printError(fmt.Sprintf("cd: %s: No such file or directory", args[1]))
			}

			if outFile != nil {
				outFile.Close()
			}
			continue
		}

		fullPath, err := exec.LookPath(command)
		if err == nil {
			cmd := exec.Command(fullPath, args[1:]...)

			cmd.Args[0] = command

			if outFile != nil {
				cmd.Stdout = outFile
			} else {
				cmd.Stdout = os.Stdout
			}

			if errFile != nil {
				cmd.Stderr = errFile
			} else {
				cmd.Stderr = os.Stderr
			}

			cmd.Stdin = os.Stdin

			_ = cmd.Run()
			continue
		}
		printError(fmt.Sprintf("%s: command not found", command))
		closeFiles()
	}
}
