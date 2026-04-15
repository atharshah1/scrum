package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
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

func PromptOptional(label string, defaultValue string) (string, error) {
	if strings.TrimSpace(defaultValue) != "" {
		fmt.Printf("%s [%s]: ", label, defaultValue)
	} else {
		fmt.Printf("%s: ", label)
	}
	in := bufio.NewReader(os.Stdin)
	text, err := in.ReadString('\n')
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(text)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
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

func PromptSelect(label string, items []string) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no options available")
	}
	p := promptui.Select{Label: label, Items: items, Size: 10}
	_, result, err := p.Run()
	if err != nil {
		return "", err
	}
	return result, nil
}
