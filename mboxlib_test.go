// Copyright (C) Michael J. Fromberger. All Rights Reserved.

package mboxlib_test

import (
	"bytes"
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

func TestScan(t *testing.T) {
	type hdrs = map[string]string
	tests := []struct {
		pos, end  int64
		bodyStart int
		headers   hdrs
		body      []string
	}{
		// The file offsets and probe strings were computed by hand.
		// If you edit the test.mbox, you will to update these test cases.
		// There must be exactly as many entries in this slice as messages.
		{0, 5011, 1590, hdrs{
			"subject": "Subversion 1.6.13 Released",
			"sender":  "hyrum@hyrumwright.org",
			"list-id": "<announce.apache.org>",
		}, []string{
			"subversion-1.6.13.tar.bz2", "5329 FCFD 6305 9821 F7B2",
		}},
		{5011, 7539, 1670, hdrs{
			"precedence": "bulk",
			"reply-to":   "jplevyak@apache.org",
		}, []string{
			"http://trafficserver.apache.org/downloads.html",
		}},
		{7539, 10737, 1382, hdrs{
			"x-spam-check-by": "apache.org",
			"to":              "announce@apache.org",
			"from":            "Niklas Gustavsson <ngn@apache.org>",
		}, []string{
			"[FTPSERVER-356] - Incorrect pom.xml on trunk",
			"The Apache MINA project\n\n", // at the end of the message
		}},
		{10737, 16212, 2478, hdrs{"date": "Mon, 4 Oct 2010 10:41:18 -0400"}, []string{
			"\n   (See CHANGES-APR-UTIL-1.3 for more information.)", // N.B. indented
		}},
		{16212, 22712, 2522, hdrs{"from": "Sally Khudairi <sk@apache.org>"}, []string{
			"Thought Leaders Dana Blankenho=\nrn of ZDNet",       // quoted-printable EOL
			"@TheASF feed on Twitter.=0A=0A# # #=0A=0A=0A      ", // spaces at EOL
		}},
		{22712, 28706, 1595, hdrs{"subject": "[ANN] Apache Maven 3.0 Released"}, []string{
			"     * [MNG-4836] - Incorrect recursive expression cycle errors (update \nplexus-interpolation)",
		}},
	}
	var i int
	for m, err := range mboxlib.Scan(bytes.NewReader(testData)) {
		if err != nil {
			t.Fatalf("index %d: unexpected error: %v", i, err)
		} else if i >= len(tests) {
			t.Errorf("index %d: unexpected extra message: %q", i, mstr.Trunc(m.Body(), 64))
			i++
			continue
		}
		tc := tests[i]

		// Check file offsets.
		if m.Pos != tc.pos || m.End != tc.end {
			t.Errorf("index %d: got offsets %d..%d, want %d..%d", i, m.Pos, m.End, tc.pos, tc.end)
		}

		// Check the body offset, and verify it gives us the same reference we
		// get from the slice into m.Data.
		if got := m.BodyOffset(); got != tc.bodyStart {
			t.Errorf("index %d: got body offset %d, want %d", i, got, tc.bodyStart)
		} else if diff := cmp.Diff(m.Body(), m.Data[got:]); diff != "" {
			t.Errorf("index %d: body offset vs data (-got, +want):\n%s", i, diff)
		}

		// Check for certain interesting headers.
		for key, want := range tc.headers {
			if got := m.ParsedHeader.Get(key); got != want {
				t.Errorf("index %d: header %q: got %q, want %q", i, key, got, want)
			}
		}

		// Check for certain interesting strings in the body.
		body := string(m.Body())
		for _, want := range tc.body {
			if !strings.Contains(body, want) {
				t.Errorf("index %d: missing body string %q", i, want)
			}
		}

		// Verify that the From_ line got populated.
		if !bytes.Contains(m.FromLine(), []byte(" MAILER-DAEMON ")) {
			t.Errorf("index %d: missing From_ line marker in %q", i, m.FromLine())
		}
		i++
	}

	// Ensure we got all the messages we expected.
	if i < len(tests) {
		t.Fatalf("Missing %d messages at EOF", len(tests)-i)
	}
}
