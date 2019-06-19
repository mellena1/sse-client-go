package sse

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

// Event is a struct holding all data from a single sse event
type Event struct {
	LastEventID string
	Type        string
	Data        []byte
}

const (
	eventTypeEvent = "event"
	eventTypeData  = "data"
	eventTypeID    = "id"
	eventTypeRetry = "retry"
)

func readEvent(data []byte) (*Event, error) {
	event := &Event{}

	if len(data) < 1 {
		return nil, errors.New("data is empty")
	}

	// make crlf into lf for the fieldsfunc to work easier
	bytes.Replace(data, []byte("\n\r"), []byte("\n"), -1)
	// Split into each line by newlines
	for _, line := range bytes.FieldsFunc(data, func(r rune) bool { return r == '\n' || r == '\r' }) {
		// Per the spec:
		// If the line starts with a U+003A COLON character (:)
		// 		Ignore the line.
		if bytes.HasPrefix(line, []byte(":")) {
			continue
		}

		var field, value []byte

		// Per the spec:
		// If the line contains a U+003A COLON character (:)
		// 		Collect the characters on the line before the first U+003A COLON character (:), and let field be that string.
		//		Collect the characters on the line after the first U+003A COLON character (:), and let value be that string. If value starts with a U+0020 SPACE character, remove it from value.
		//		Process the field using the steps described below, using field as the field name and value as the field value.
		if bytes.Contains(line, []byte(":")) {
			splitLine := bytes.Split(line, []byte(":"))
			field = splitLine[0]
			value = splitLine[1]
			// trim space from beginning of value
			value = bytes.TrimPrefix(value, []byte(" "))
		} else {
			// Per the spec:
			// Otherwise, the string is not empty but does not contain a U+003A COLON character (:)
			// 		Process the field using the steps described below,
			// 		using the whole line as the field name, and the empty string as the field value.
			field = line
			value = []byte("")
		}

		switch {
		case bytes.Equal(field, []byte(eventTypeEvent)):
			// Set the event type buffer to field value.
			event.Type = string(value)
		case bytes.Equal(field, []byte(eventTypeData)):
			// Append the field value to the data buffer,
			// then append a single U+000A LINE FEED (LF) character to the data buffer.
			event.Data = append(value, []byte("\n")...)
		case bytes.Equal(field, []byte(eventTypeID)):
			// If the field value does not contain U+0000 NULL,
			// then set the last event ID buffer to the field value.
			if !bytes.Contains(value, []byte("\000")) {
				event.LastEventID = string(value)
			}
			// Otherwise, ignore the field.
		case bytes.Equal(field, []byte(eventTypeRetry)):
			// TODO: Unimplemented currently
		default:
			// ignore the line
		}
	}

	// Per the spec:
	// If the data buffer's last character is a U+000A LINE FEED (LF) character,
	// then remove the last character from the data buffer.
	event.Data = bytes.TrimSuffix(event.Data, []byte("\n"))

	return event, nil
}

// eventScannerFunc function to use for the event scanner
// An event is complete when there is an empty line, so two line endings signals the end of the event
//
// As per the spec:
// The stream must then be parsed by reading everything line by line,
// with a U+000D CARRIAGE RETURN U+000A LINE FEED (CRLF) character pair,
// a single U+000A LINE FEED (LF) character not preceded by a U+000D CARRIAGE RETURN (CR) character,
// and a single U+000D CARRIAGE RETURN (CR) character not followed by a U+000A LINE FEED (LF) character
// being the ways in which a line can end.
var eventScannerFunc bufio.SplitFunc = func(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// reader has no more data
	if atEOF {
		return len(data), data, nil
	}

	// a U+000D CARRIAGE RETURN U+000A LINE FEED (CRLF) character pair
	if i := bytes.Index(data, []byte("\r\n\r\n")); i >= 0 {
		return i + 1, data[0:i], nil
	}
	// a single U+000A LINE FEED (LF) character not preceded by a U+000D CARRIAGE RETURN (CR) character
	if i := bytes.Index(data, []byte("\n\n")); i >= 0 {
		return i + 1, data[0:i], nil
	}
	// a single U+000D CARRIAGE RETURN (CR) character not followed by a U+000A LINE FEED (LF) character
	if i := bytes.Index(data, []byte("\r\r")); i >= 0 {
		return i + 1, data[0:i], nil
	}

	// didn't find the end of a line
	return 0, nil, nil
}

type eventScanner struct {
	*bufio.Scanner
}

func newEventScanner(body io.Reader) *eventScanner {
	scanner := bufio.NewScanner(body)
	scanner.Split(eventScannerFunc)
	return &eventScanner{scanner}
}

func (scanner *eventScanner) scanEvent() ([]byte, error) {
	if scanner.Scan() {
		return scanner.Bytes(), nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}
