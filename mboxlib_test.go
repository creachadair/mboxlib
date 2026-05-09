// Copyright (C) Michael J. Fromberger. All Rights Reserved.

package mboxlib_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/creachadair/mboxlib"
	"github.com/creachadair/mds/mstr"
	"github.com/google/go-cmp/cmp"

	_ "embed"
)

// Test e-mails extracted from the Apache public announce list:
// https://lists.apache.org/list?announce@apache.org:2010-10
//
//go:embed testdata/test.mbox
var testData []byte

type hdrs = map[string]string

type testCase struct {
	pos, end    int64 // byte offsets, 0-based
	bodyStart   int
	first, last int // line numbers, 1-based
	headers     hdrs
	body        []string
}

var tests = []testCase{
	// The file offsets and probe strings were computed by hand.
	// If you edit the test.mbox, you will to update these test cases.
	// There must be exactly as many entries in this slice as messages.
	{0, 5011, 1590, 1, 108, hdrs{
		"subject": "Subversion 1.6.13 Released",
		"sender":  "hyrum@hyrumwright.org",
		"list-id": "<announce.apache.org>",
	}, []string{
		"subversion-1.6.13.tar.bz2", "5329 FCFD 6305 9821 F7B2",
	}},
	{5011, 7539, 1670, 109, 168, hdrs{
		"precedence": "bulk",
		"reply-to":   "jplevyak@apache.org",
	}, []string{
		"http://trafficserver.apache.org/downloads.html",
	}},
	{7539, 10737, 1382, 169, 244, hdrs{
		"x-spam-check-by": "apache.org",
		"to":              "announce@apache.org",
		"from":            "Niklas Gustavsson <ngn@apache.org>",
	}, []string{
		"[FTPSERVER-356] - Incorrect pom.xml on trunk",
		"The Apache MINA project\n\n", // at the end of the message
	}},
	{10737, 16212, 2478, 245, 369, hdrs{"date": "Mon, 4 Oct 2010 10:41:18 -0400"}, []string{
		"\n   (See CHANGES-APR-UTIL-1.3 for more information.)", // N.B. indented
	}},
	{16212, 22712, 2522, 370, 470, hdrs{"from": "Sally Khudairi <sk@apache.org>"}, []string{
		"Thought Leaders Dana Blankenho=\nrn of ZDNet",       // quoted-printable EOL
		"@TheASF feed on Twitter.=0A=0A# # #=0A=0A=0A      ", // spaces at EOL
	}},
	{22712, 28706, 1595, 471, 607, hdrs{"subject": "[ANN] Apache Maven 3.0 Released"}, []string{
		"     * [MNG-4836] - Incorrect recursive expression cycle errors (update \nplexus-interpolation)",
	}},
}

func (tc testCase) checkMessage(t *testing.T, m *mboxlib.Message) {
	t.Helper()
	// Check the body offset, and verify it gives us the same reference we
	// get from the slice into m.Data.
	if got := m.BodyOffset(); got != tc.bodyStart {
		t.Errorf("BodyOffset = %d, want %d", got, tc.bodyStart)
	} else if diff := cmp.Diff(m.Body(), m.Data[got:]); diff != "" {
		t.Errorf("Computed body offset vs data (-got, +want):\n%s", diff)
	}

	// Check for certain interesting headers.
	for key, want := range tc.headers {
		if got := m.ParsedHeader.Get(key); got != want {
			t.Errorf("Check header %q: got %q, want %q", key, got, want)
		}
	}

	// Check for certain interesting strings in the body.
	body := string(m.Body())
	for _, want := range tc.body {
		if !strings.Contains(body, want) {
			t.Errorf("Missing body string %q", want)
		}
	}

	// Verify that the From_ line got populated.
	if !bytes.Contains(m.FromLine(), []byte(" MAILER-DAEMON ")) {
		t.Errorf("Missing From_ line marker in %q", m.FromLine())
	}
}

func TestScanner(t *testing.T) {
	s := mboxlib.NewScanner(bytes.NewReader(testData))
	var i int
	for {
		data, err := s.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("index %d: unexpected error: %v", i, err)
		} else if i >= len(tests) {
			t.Errorf("index %d: unexpected extra message: %q", i, mstr.Trunc(data, 64))
		}
		tc := tests[i]

		// Check that the scanner reports the expected offsets.
		pos, end := s.Span()
		first, last := s.Lines()
		t.Logf("index %d: message %d..%d lines %d..%d (%d bytes)", i, pos, end, first, last, len(data))

		if pos != tc.pos || end != tc.end {
			t.Errorf("index %d: got offsets %d..%d, want %d..%d", i, pos, end, tc.pos, tc.end)
		}
		if first != tc.first || last != tc.last {
			t.Errorf("index %d: got lines %d..%d, want %d..%d", i, first, last, tc.first, tc.last)
		}

		msg, err := mboxlib.ParseMessage(data)
		if err != nil {
			t.Fatalf("index %d: parse message failed: %v", i, err)
		}
		tc.checkMessage(t, msg)
		i++
	}
	// Ensure we got all the messages we expected.
	if i < len(tests) {
		t.Fatalf("Missing %d messages at EOF", len(tests)-i)
	}
}

func TestScan(t *testing.T) {
	var i int
	for m, err := range mboxlib.Scan(bytes.NewReader(testData)) {
		if err != nil {
			t.Fatalf("index %d: unexpected error: %v", i, err)
		} else if i >= len(tests) {
			t.Errorf("index %d: unexpected extra message: %q", i, mstr.Trunc(m.Body(), 64))
			i++
			continue
		}
		t.Logf("index %d: subject %q (%d bytes)", i, m.ParsedHeader.Get("subject"), len(m.Data))
		tests[i].checkMessage(t, m)
		i++
	}

	// Ensure we got all the messages we expected.
	if i < len(tests) {
		t.Fatalf("Missing %d messages at EOF", len(tests)-i)
	}
}
