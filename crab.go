package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	colour "github.com/fatih/color"
)

const (
	path     = "store.json" // path to store formed tree to
	maxDepth = 100
	cloak    = "Mozilla/5.0 (X11; Linux x86_64; rv:108.0) Gecko/20100101 Firefox/108.0" // cloak user-agent
	method   = "HEAD"
)

var (
	urls []*url.URL
	//continue_ bool

	// http client to be used
	client = http.DefaultClient

	// colour printers
	errorPrinter = colour.New(colour.FgHiRed)
	//successPrinter = colour.New(colour.FgHiGreen)

	// regex for urls
	urlScanner *regexp.Regexp

	// the url tree
	tree []*UrlTree
	past []string
)

// holds a url and the urls found within
type UrlTree struct {
	Url      string     `json:"url"`
	Continue bool       `json:"continue,omitempty"`
	Trees    []*UrlTree `json:"trees"`
}

// saves all progress
func save() {
	b, err := json.MarshalIndent(tree, "", "    ") // gen json
	if err != nil {
		errorPrinter.Println("error parsing url tree to json.")
		os.Exit(1)
	}
	os.WriteFile(path, b, 0644)
}

func inPast(url string) bool {
	for _, u := range past {
		su := strings.Split(u, "//")
		sp := strings.Split(url, "//")
		if su[len(su)-1] == sp[len(sp)-1] {
			return true
		}
	}
	return false
}

func main() {
	// ensure output is written before exit
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		errorPrinter.Println("halting...")
		save()
		os.Exit(1)
	}()

	var search func(utr *UrlTree)
	search = func(utr *UrlTree) {
		// get data
		req, err := http.NewRequest("GET", utr.Url, nil)
		req.Header.Set("User-Agent", cloak)
		if err != nil {
			errorPrinter.Println("failed to form request.")
			return
		}
		// send request
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			errorPrinter.Println("failed to send request.")
			return
		}
		// get body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			errorPrinter.Println("error reading body.")
			return
		}
		// find all urls
		for _, uri := range urlScanner.FindAllString(string(body), -1) {
			if inPast(uri) {
				continue
			}
			utn := UrlTree{Url: uri}
			utr.Trees = append(utr.Trees, &utn)
			past = append(past, uri) // add to past
			search(&utn)
		}
	}

	// form urltrees for every url
	for _, uri := range urls {
		if inPast(uri.String()) {
			continue
		}
		utr := UrlTree{Url: uri.String()}
		tree = append(tree, &utr)
		past = append(past, utr.Url) // add to past
		search(&utr)
	}

	// save results
	save()
}

func init() {
	// create regex urlscanner
	urlScanner, _ = regexp.Compile(`https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`)

	// get urls
	args := os.Args[1:]
	if len(args) < 1 {
		errorPrinter.Println("no url(s) passed.")
		os.Exit(1)
	} else {
		// continue from last
		if strings.ToLower(args[0]) == "continue" {
			//continue_ = true
		} else {
			urls = make([]*url.URL, len(args))
			for i, arg := range args {
				// attempt to parse url
				uri, err := url.Parse(arg)
				if err != nil {
					errorPrinter.Printf("error parsing url: '%s'.\n", arg)
					os.Exit(1)
				}
				urls[i] = uri
			}
		}
	}

	// ensure all urls are valid
	for _, uri := range urls {
		urist := uri.String() // get raw url
		req, err := http.NewRequest(method, urist, nil)
		req.Header.Set("User-Agent", cloak)
		if err != nil {
			errorPrinter.Printf("failed to form %s request to url: '%s'.\n", method, urist)
			os.Exit(1)
		}
		// send request
		resp, err := client.Do(req)
		if err != nil {
			errorPrinter.Printf("failed to send %s request to url: '%s'.\n", method, urist)
			os.Exit(1)
		} else if resp.StatusCode < 200 || resp.StatusCode > 299 {
			errorPrinter.Printf("received bad whilst sending %s request: '%d'.\n", method, resp.StatusCode)
			os.Exit(1)
		}
	}
}
