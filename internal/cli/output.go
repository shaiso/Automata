package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// Output управляет форматированием вывода CLI.
type Output struct {
	jsonMode bool
	w        io.Writer // stdout для данных
	errW     io.Writer // stderr для сообщений
}

// NewOutput создаёт Output. Если jsonMode=true, данные выводятся в JSON.
func NewOutput(jsonMode bool) *Output {
	return &Output{
		jsonMode: jsonMode,
		w:        os.Stdout,
		errW:     os.Stderr,
	}
}

// Print выводит данные: таблицу или JSON в зависимости от режима.
func (o *Output) Print(headers []string, rows [][]string, jsonData any) {
	if o.jsonMode {
		o.JSON(jsonData)
		return
	}
	o.Table(headers, rows)
}

// Table выводит данные в виде таблицы через tabwriter.
func (o *Output) Table(headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(o.w, 0, 0, 2, ' ', 0)

	// Заголовки
	fmt.Fprintln(tw, strings.Join(headers, "\t"))

	// Разделитель
	dashes := make([]string, len(headers))
	for i, h := range headers {
		dashes[i] = strings.Repeat("-", len(h))
	}
	fmt.Fprintln(tw, strings.Join(dashes, "\t"))

	// Строки данных
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}

	tw.Flush()
}

// JSON выводит данные в формате JSON с отступами.
func (o *Output) JSON(v any) {
	enc := json.NewEncoder(o.w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// Success выводит сообщение об успехе в stderr.
func (o *Output) Success(msg string) {
	fmt.Fprintln(o.errW, msg)
}

// Error выводит сообщение об ошибке в stderr.
func (o *Output) Error(msg string) {
	fmt.Fprintln(o.errW, "Error: "+msg)
}
