package service

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	kiroEventHeaderStringType = 7
)

type kiroEventStreamFrame struct {
	MessageType   string
	EventType     string
	ExceptionType string
	ErrorCode     string
	Payload       []byte
}

type kiroEventStreamDecoder struct {
	reader *bufio.Reader
}

func newKiroEventStreamDecoder(r io.Reader) *kiroEventStreamDecoder {
	return &kiroEventStreamDecoder{
		reader: bufio.NewReaderSize(r, 64*1024),
	}
}

func (d *kiroEventStreamDecoder) Decode() (*kiroEventStreamFrame, error) {
	prelude := make([]byte, 12)
	if _, err := io.ReadFull(d.reader, prelude); err != nil {
		return nil, err
	}

	totalLength := int(binary.BigEndian.Uint32(prelude[0:4]))
	headersLength := int(binary.BigEndian.Uint32(prelude[4:8]))
	if totalLength < 16 || headersLength < 0 || headersLength > totalLength-16 {
		return nil, fmt.Errorf("invalid kiro event stream frame")
	}

	restLength := totalLength - len(prelude)
	rest := make([]byte, restLength)
	if _, err := io.ReadFull(d.reader, rest); err != nil {
		return nil, err
	}

	payloadLength := restLength - headersLength - 4
	if payloadLength < 0 {
		return nil, fmt.Errorf("invalid kiro event stream payload length")
	}

	headers, err := parseKiroEventStreamHeaders(rest[:headersLength])
	if err != nil {
		return nil, err
	}

	payload := rest[headersLength : headersLength+payloadLength]
	return &kiroEventStreamFrame{
		MessageType:   headers[":message-type"],
		EventType:     headers[":event-type"],
		ExceptionType: headers[":exception-type"],
		ErrorCode:     headers[":error-code"],
		Payload:       payload,
	}, nil
}

func parseKiroEventStreamHeaders(raw []byte) (map[string]string, error) {
	headers := make(map[string]string)
	for offset := 0; offset < len(raw); {
		nameLength := int(raw[offset])
		offset++
		if offset+nameLength+1 > len(raw) {
			return nil, fmt.Errorf("invalid kiro event stream header name")
		}

		name := string(raw[offset : offset+nameLength])
		offset += nameLength

		valueType := raw[offset]
		offset++
		if valueType != kiroEventHeaderStringType {
			return nil, fmt.Errorf("unsupported kiro event stream header type: %d", valueType)
		}
		if offset+2 > len(raw) {
			return nil, fmt.Errorf("invalid kiro event stream header length")
		}

		valueLength := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
		offset += 2
		if offset+valueLength > len(raw) {
			return nil, fmt.Errorf("invalid kiro event stream header value")
		}

		headers[name] = string(raw[offset : offset+valueLength])
		offset += valueLength
	}
	return headers, nil
}

func isKiroEventStreamContentType(header http.Header) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(header.Get("Content-Type"))), "application/vnd.amazon.eventstream")
}

func frameToKiroEventMap(frame *kiroEventStreamFrame) (map[string]any, error) {
	if frame == nil {
		return nil, nil
	}

	switch frame.MessageType {
	case "event":
		event := make(map[string]any)
		if len(frame.Payload) > 0 {
			if err := json.Unmarshal(frame.Payload, &event); err != nil {
				return nil, err
			}
		}
		event["type"] = frame.EventType
		return event, nil
	case "exception":
		return map[string]any{
			"type":          "exception",
			"exceptionType": frame.ExceptionType,
			"message":       strings.TrimSpace(string(frame.Payload)),
		}, nil
	case "error":
		return map[string]any{
			"type":          "exception",
			"exceptionType": frame.ErrorCode,
			"message":       strings.TrimSpace(string(frame.Payload)),
		}, nil
	default:
		return nil, nil
	}
}
