package client

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// readLine lê uma linha de entrada do usuário, exibindo um prompt e retornando a string lida.
func readLine(input *bufio.Reader, output io.Writer, prompt string) (string, error) {
	fmt.Fprint(output, prompt)
	value, err := input.ReadString('\n')

	if err != nil && err != io.EOF {
		return "", err
	}

	return strings.TrimSpace(value), nil
}

// waitForEnter mantém o resultado de uma operação visível até o usuário voltar ao menu.
func waitForEnter(input *bufio.Reader, output io.Writer) bool {
	_, err := readLine(input, output, "\nPressione Enter para continuar...")
	return err == nil
}

// chooseNumber lê uma opção numérica entre zero e maximum. Zero é reservado
// para cancelar a seleção sem executar uma operação.
func chooseNumber(input *bufio.Reader, output io.Writer, prompt string, maximum int) (int, bool) {
	for {
		text, err := readLine(input, output, prompt)
		if err != nil {
			return 0, false
		}

		value, err := strconv.Atoi(text)
		if err != nil || value < 0 || value > maximum {
			fmt.Fprintf(output, "Escolha um número entre 0 e %d.\n", maximum)
			continue
		}

		return value, true
	}
}
