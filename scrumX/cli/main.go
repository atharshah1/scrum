package main

import (
"fmt"
"os"

"github.com/atharshah1/scrum/scrumX/cli/cmd"
)

func main() {
if err := cmd.Execute(); err != nil {
fmt.Fprintln(os.Stderr, err)
os.Exit(1)
}
}
