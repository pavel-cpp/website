// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Screentest compares images of rendered web pages.
It compares images obtained from two sources, one to test and one for the expected
result. The comparisons are driven by a script file in a format described below.

# Usage

	screentest [flags] testURL wantURL file ...

The first two arguments are the URLs being tested and the URL of the desired result,
respectively. The remaining arguments are script file pathnames.

The URLs can be actual URLs for http:, https:, file: or gs: schemes, or they
can be slash-separated file paths (even on Windows).

The flags are:

	 -c
		  Number of test cases to run concurrently.
	-d
		  URL of a Chrome websocket debugger. If omitted, screentest tries to find the
		  Chrome executable on the system and starts a new instance.
	-headers
		  HTTP(S) headers to send with each request, as a comma-separated list of name:value.
	-run REGEXP
		  Run only tests matching regexp.
	 -o
		  URL or slash-separated path for output files. If omitted, files are written
		  to a subdirectory of the user's cache directory. Each test file is given
		  its own directory, so test names in two files can be identical. But the directory
		  name is the basename of the test file with the extension removed. Conflicting
		  file names will overwrite each other.
	 -u
		  Instead of comparing screenshots, use the test screenshots to update the
		  want screenshots. This only makes sense if wantURL is a storage location
		  like a file path or GCS bucket.
	 -v
		  Variables provided to script templates as comma separated KEY:VALUE pairs.

# Scripts

A script file contains one or more test cases described as a sequence of lines. The
file is first processed as Go template using the text/template package, with a map
of the variables given by the -v flag provided as `.`.

The script format is line-oriented.
Lines beginning with # characters are ignored as comments.
Each non-blank, non-comment line is a directive, listed below.

Each test case begins with the 'test' directive and ends with a blank line.
A test case describes actions to take on a page, along
with the dimensions of the screenshots to be compared. For example, here is
a trivial script:

	test about
	pathname /about
	capture fullscreen

This script has a single test case. The first line names the test.
The second line sets the page to visit at each origin. The last line
captures full-page screenshots of the pages and generates a diff image if they
do not match.

# Directives

Use windowsize WIDTHxHEIGHT to set the default window size for all test cases
that follow.

	windowsize 540x1080

Use block URL ... to set URL patterns to block. Wildcards ('*') are allowed.

	block https://codecov.io/* https://travis-ci.com/*

The directives above apply to all test cases that follow.
The ones below must appear inside a test case and apply only to that case.

Use test NAME to create a name for the test case.

	test about page

Use pathname PATH to set the page to visit at each origin.

	pathname /about

Use status CODE to set an expected HTTP status code. The default is 200.

	status 404

Use click SELECTOR to add a click an element on the page.

	click button.submit

Use wait SELECTOR to wait for an element to appear.

	wait [role="treeitem"][aria-expanded="true"]

Use eval JS to evaluate JavaScript snippets to hide elements or prepare the page in
some other way.

	eval 'document.querySelector(".selector").remove();'
	eval 'window.scrollTo({top: 0});'

Use capture [SIZE] [ARG] to create a test case with the properties
defined in the test case. If present, the first argument to capture must be one of
'fullscreen', 'viewport' or 'element'. The optional second argument provides
a viewport size. The defaults are 'viewport' with dimensions specified by the windowsize
directive.

	capture fullscreen 540x1080

When taking an element screenshot provide a selector.

	capture element header

Each capture directive creates a new test case for a single page.

	windowsize 1536x960

	test homepage
	pathname /
	capture viewport
	capture viewport 540x1080
	capture viewport 400x1000

	test about page
	pathname /about
	capture viewport
	capture viewport 540x1080
	capture viewport 400x1000
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
)

var flags options

func init() {
	flag.BoolVar(&flags.update, "u", false, "update want with test")
	flag.StringVar(&flags.vars, "v", "", "variables provided to script templates as comma separated KEY:VALUE pairs")
	flag.IntVar(&flags.maxConcurrency, "c", (runtime.NumCPU()+1)/2, "number of test cases to run concurrently")
	flag.StringVar(&flags.debuggerURL, "d", "", "chrome debugger URL")
	flag.StringVar(&flags.outputDirURL, "o", "", "path for output: file path or URL with 'file' or 'gs' scheme")
	flag.StringVar(&flags.headers, "headers", "", "HTTP headers: comma-separated list of name:value")
	flag.StringVar(&flags.filterRegexp, "run", "", "regexp to match test")
}

// options are the options for the program.
// See the top of this file and the flag.XXXVar calls above for documentation.
type options struct {
	update         bool
	vars           string
	maxConcurrency int
	debuggerURL    string
	filterRegexp   string
	outputDirURL   string
	headers        string
}

func main() {
	flag.Usage = func() {
		fmt.Printf("usage: screentest [flags] testURL wantURL path ...\n")
		fmt.Printf("\ttestURL is the URL or file path to be tested\n")
		fmt.Printf("\twantURL is the URL or file path to compare it to\n")
		fmt.Printf("\teach path is a script file to execute\n")

		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 3 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(context.Background(), flag.Arg(0), flag.Arg(1), flag.Args()[2:], flags); err != nil {
		log.Fatal(err)
	}
}
