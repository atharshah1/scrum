package utils

import (
"bufio"
"fmt"
"os"
"strings"

"golang.org/x/term"
)

func Prompt(label string) (string, error) {
fmt.Printf("%s: ", label)
in := bufio.NewReader(os.Stdin)
text, err := in.ReadString('\n')
if err != nil {
return "", err
}
return strings.TrimSpace(text), nil
}

func PromptPassword(label string) (string, error) {
fmt.Printf("%s: ", label)
pw, err := term.ReadPassword(int(os.Stdin.Fd()))
fmt.Println()
if err != nil {
return "", err
}
return strings.TrimSpace(string(pw)), nil
}
