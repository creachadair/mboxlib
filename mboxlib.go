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
			m.Pos, m.End = s.pos, s.end
			if !yield(m, nil) {
				return
			}
		}
	}
}

// A Scanner is a structural scanner for a Unix mailbox (mbox) file.
type Scanner struct {
	r        io.Reader
	buf      bytes.Buffer
	cur      []byte
	line     int   // first line of cur, 1-based
	pos, end int64 // file offsets, 0-based
}

// NewScanner constructs a [Scanner] that consumes the contents of r in Unix mailbox (mbox) format.
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{r: bufio.NewReader(r), line: 1}
}

// Next parses and returns the next available raw message from s, or reports an error.
// At the end of input, it returns [io.EOF].
func (s *Scanner) Next() ([]byte, error) {
	s.line += bytes.Count(s.cur, []byte("\n"))
	s.pos = s.end
	s.cur = nil

	for {
		i := bytes.Index(s.buf.Bytes(), []byte("\nFrom "))
		if i < 0 {
			nr, err := io.Copy(&s.buf, io.LimitReader(s.r, 1<<20))
			if nr == 0 {
				break
			} else if err != nil {
				return nil, err
			}
			continue
		}

		s.cur = s.buf.Next(i + 1) // +1 for the newline
		s.end += int64(len(s.cur))
		return s.cur, nil
	}

	// Reaching here, we saw io.EOF from the reader.
	n := s.buf.Len()
	if n == 0 {
		return nil, io.EOF
	}
	s.cur = s.buf.Next(n)
	s.end += int64(len(s.cur))
	return s.cur, nil
}

// A Message is the parsed representation of a mail message from an mbox file.
type Message struct {
	// ParsedHeader contains the parsed mail headers.
	ParsedHeader mail.Header

	// Data contains the raw, unparsed message as-read from the input.
	// It includes the From_ line, if one was present.
	Data []byte

	// The 0-based byte offsets of Data in the original input.
	Pos, End int64

	fromLine  []byte // a prefix of Data containing the From_ line
	rawHeader []byte // a slice of Data containing the header
	rawBody   []byte // a slice of Data containing the body
}

// ParseMessage parses a [Message] from the specified raw message data.
// The Pos and End fields of a successful result are set to 0.
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
