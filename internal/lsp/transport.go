package lsp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const maxMessageBytes = 8 << 20

func readFrame(reader *bufio.Reader) ([]byte, error) {
	length := -1
	headerBytes := 0
	for {
		line, err := reader.ReadSlice('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && headerBytes == 0 && len(line) == 0 {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("incomplete LSP header: %w", err)
		}
		headerBytes += len(line)
		if headerBytes > 8192 || len(line) < 2 || line[len(line)-2] != '\r' {
			return nil, fmt.Errorf("invalid LSP header line")
		}
		if len(line) == 2 {
			break
		}
		key, value, ok := strings.Cut(string(line[:len(line)-2]), ":")
		if !ok {
			return nil, fmt.Errorf("invalid LSP header field")
		}
		if !strings.EqualFold(key, "Content-Length") {
			continue
		}
		if length >= 0 {
			return nil, fmt.Errorf("duplicate Content-Length")
		}
		value = strings.TrimSpace(value)
		if value == "" || strings.Trim(value, "0123456789") != "" {
			return nil, fmt.Errorf("invalid Content-Length")
		}
		length, err = strconv.Atoi(value)
		if err != nil || length < 1 || length > maxMessageBytes {
			return nil, fmt.Errorf("Content-Length is outside the supported range")
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	message := make([]byte, length)
	if _, err := io.ReadFull(reader, message); err != nil {
		return nil, fmt.Errorf("incomplete LSP body: %w", err)
	}
	return message, nil
}

func writeFrame(writer io.Writer, message []byte) error {
	if len(message) == 0 || len(message) > maxMessageBytes {
		return fmt.Errorf("LSP message size is outside the supported range")
	}
	if err := writeAll(writer, []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(message)))); err != nil {
		return err
	}
	return writeAll(writer, message)
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(data) {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
