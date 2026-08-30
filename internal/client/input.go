package client

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func readLine(input *bufio.Reader, output io.Writer, prompt string) (string, error) {
	fmt.Fprint(output, prompt)
	value, err := input.ReadString('\n')

	if err != nil && err != io.EOF {
		return "", err
	}

	return strings.TrimSpace(value), nil
}
