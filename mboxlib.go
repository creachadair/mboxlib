// Copyright (C) Michael J. Fromberger. All Rights Reserved.

// Package mboxlib implements basic iteration and parsing of Unix mailbox
// (mbox) format files. Presently, only the "classic" format is supported,
// using "From_" lines to delimit messages.
package mboxlib

import (
	"bufio"
	"bytes"
	"io"
	"iter"
	"net/mail"
)

// Scan returns an iterator over the contents of r as a sequence of [Message]
// values.  Each reported pair has either a valid message and a nil error, or a
// nil message and a non-nil error. After an error occurs, the iterator stops.
func Scan(r io.Reader) iter.Seq2[*Message, error] {
	return func(yield func(*Message, error) bool) {
		s := NewScanner(r)
		for {
			next, err := s.Next()
			if err == io.EOF {
				return // no more messages
			} else if err != nil {
				yield(nil, err)
				return
			}
			m, err := ParseMessage(next)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(m, nil) {
				return
			}
		}
	}
}

// A Scanner is a structural scanner for a Unix mailbox (mbox) file.
type Scanner struct {
	r           io.Reader
	buf         bytes.Buffer
	cur         []byte
	first, last int   // first and last lines of cur, 1-based
	pos, end    int64 // file offsets, 0-based
}

// NewScanner constructs a [Scanner] that consumes the contents of r in Unix mailbox (mbox) format.
func NewScanner(r io.Reader) *Scanner { return &Scanner{r: bufio.NewReader(r)} }

// Next parses and returns the next available raw message from s, or reports an error.
// At the end of input, it returns [io.EOF].
func (s *Scanner) Next() ([]byte, error) {
	s.first = s.last + 1
	s.pos = s.end
	s.cur = nil

	for {
		i := bytes.Index(s.buf.Bytes(), []byte("\nFrom "))
		if i < 0 {
			nr, err := io.Copy(&s.buf, io.LimitReader(s.r, 1<<20))
			if err != nil {
				return nil, err
			} else if nr == 0 {
				break
			}
			continue
		}

		s.cur = s.buf.Next(i + 1) // +1 for the newline
		s.end += int64(len(s.cur))
		s.last = s.first + countLines(s.cur) - 1 // -1 so we don't double-count first
		return s.cur, nil
	}

	// Reaching here, we saw io.EOF from the reader.
	n := s.buf.Len()
	if n == 0 {
		return nil, io.EOF
	}
	s.cur = s.buf.Next(n)
	s.end += int64(len(s.cur))
	s.last = s.first + countLines(s.cur) - 1 // -1 so we don't double-count first
	return s.cur, nil
}

// Span reports the starting and ending offsets (0-based) of the most recent
// message reported by a successful call to [Scanner.Next] in the input.
// If Next has not been called, it returns 0, 0.
func (s *Scanner) Span() (from, to int64) { return s.pos, s.end }

// Lines reports the starting and ending line numbers (1-based) of the most
// recent message reporte by a successful call to [Scanner.Next] in the input.
// If Next has not been called, it returns 0, 0.
func (s *Scanner) Lines() (first, last int) { return s.first, s.last }

// A Message is the parsed representation of a mail message from an mbox file.
type Message struct {
	// ParsedHeader contains the parsed mail headers.
	ParsedHeader mail.Header

	// Data contains the raw, unparsed message as-read from the input.
	// It includes the From_ line, if one was present.
	Data []byte

	fromLine  []byte // a prefix of Data containing the From_ line
	rawHeader []byte // a slice of Data containing the header
	rawBody   []byte // a slice of Data containing the body
}

// ParseMessage parses a [Message] from the specified raw message data.
func ParseMessage(data []byte) (*Message, error) {
	var fromLine, rest []byte = nil, data
	if bytes.HasPrefix(data, []byte("From ")) {
		fromLine, rest = cutAfter(data, []byte("\n"))
	}

	cr := &countReader{data: rest}
	msg, err := mail.ReadMessage(cr)
	if err != nil {
		return nil, err
	}

	extra := msg.Body.(*bufio.Reader).Buffered()
	endFrom := len(fromLine)
	bodyOffset := (cr.numRead - extra) + endFrom
	return &Message{
		ParsedHeader: msg.Header,
		Data:         data,

		fromLine:  fromLine,
		rawHeader: data[endFrom:bodyOffset],
		rawBody:   data[bodyOffset:],
	}, nil
}

// Header returns a slice into m.Data containing the unparsed message header.
func (m *Message) Header() []byte { return m.rawHeader }

// Body returns a slice into m.Data containing the unparsed message body.
func (m *Message) Body() []byte { return m.rawBody }

// BodyOffset reports the offset into m.Data where the message body begins.
func (m *Message) BodyOffset() int { return len(m.Data) - len(m.rawBody) }

// FromLine returns a slice into m.Data containing the unparsed From_ line.
// Leading and trailing whitespace is trimmed from the result.
func (m *Message) FromLine() []byte { return bytes.TrimSpace(m.fromLine) }

type countReader struct {
	data    []byte
	numRead int
}

func (c *countReader) Read(out []byte) (int, error) {
	if len(c.data) == 0 {
		return 0, io.EOF
	}
	nr := min(len(c.data), len(out))
	copy(out, c.data[:nr])
	c.data = c.data[nr:]
	c.numRead += nr
	return nr, nil
}

func cutAfter(s, sep []byte) (first, rest []byte) {
	i := bytes.Index(s, sep)
	if i < 0 {
		return s, nil
	}
	end := i + len(sep)
	return s[:end], s[end:]
}

// countLines reports the number of lines in data, where line is a maximal
// contiguous span of zero or more bytes not containing a newline, and ending
// with a newline or the end of the input.
func countLines(data []byte) int {
	if len(data) == 0 {
		return 1
	}
	n := bytes.Count(data, []byte("\n"))
	if len(data) != 0 && data[len(data)-1] != '\n' {
		n++
	}
	return n
}
