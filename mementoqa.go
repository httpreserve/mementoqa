package main

import (
	"encoding/json"
	ps "github.com/httpreserve/phantomjsscreenshot"
	sr "github.com/httpreserve/simplerequest"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

var screenshot = false
var port = "8080"

func init() {
	if ps.Hello() {
		screenshot = true
	}
}

var useragent = "memento-qa/0.0.1"

type timemap struct {
	id         string
	uri        string
	resolved   bool
	compliant  bool
	original   string
	timegate   string
	screenshot string
	captured   bool
}

var memes []timemap

const datelayout = "20060102150405"

func getPotentialURL(archiveurl string, uri string, date string) string {
	base := strings.Split(archiveurl, "timemap")[0]
	return base + date + "/" + uri
}

const wedoitbase = "http://labs.mementoweb.org/timemap/json/"
const base = "http://timetravel.mementoweb.org/timemap/json/"

func makeTimemap(uri string) string {
	return base + uri
}

func recurseInterface(f interface{}) {

	var tm timemap

	m := f.(map[string]interface{})
	for k, v := range m {
		switch value := v.(type) {
		case string:
			v := v.(string)
			switch k {
			case "uri":
				u, err := url.Parse(v)
				tm.uri = u.String()
				if err != nil {
					log.Println("Problem parsing URI sent by time travel. URI will be blank.", v)
					tm.uri = ""
				}
			case "archive_id":
				tm.id = v
			case "memento_compliant":
				if v == "no" {
					tm.compliant = false
				} else if v == "yes" {
					tm.compliant = true
				} else {
					log.Println("unknown compliance status")
				}
			case "original_uri":
				tm.original = v
			case "timegate_uri":
				tm.timegate = v
			default:
				log.Println("unknown API function, check memento version", k, v)
			}
		case int:
			log.Println("int, in json unmarshal interface")
		case []interface{}:
			//we're expecting this, recurse and decode...
			for _, u := range value {
				recurseInterface(u)
			}
			//default:
			//we've information we haven't worked with before...
		}
	}

	if tm.uri != "" {
		memes = append(memes, tm)
	}
}

func manageURIExceptions(oldURI, newURI string) string {

	if strings.Contains(oldURI, "http://wayback.archive-it.org/all/") {
		s := strings.Split(oldURI, "/all/")[0]
		return s + newURI
	}
	return newURI
}

func addDateURIs(mementos []timemap, url string, date string) {
	for i := range mementos {
		if mementos[i].uri != "" {
			if mementos[i].compliant {
				uri := getPotentialURL(mementos[i].uri, url, date)
				resp, err := makeSimpleRequest(uri)
				if err != nil {
					log.Println("error making simplerequest for archival uri")
				} else {
					tmpURI := resp.GetHeader("Location")
					if tmpURI != "" {
						mementos[i].uri = manageURIExceptions(uri, tmpURI)
						mementos[i].resolved = true
					}
				}
			}
		}
	}

	log.Println("Results for:", len(mementos), "potential mementos")
}

func getTimemap(uri string) ([]timemap, error) {
	resp, _ := makeSimpleRequest(uri)
	var m interface{}
	err := json.Unmarshal([]byte(resp.Data), &m)
	if err != nil {
		return nil, err
	}
	recurseInterface(m)
	return memes, nil
}

func makeSimpleRequest(uri string) (sr.SimpleResponse, error) {
	req, err := sr.Create(sr.GET, uri)
	if err != nil {
		return sr.SimpleResponse{}, err
	}
	req.NoRedirect(true)
	req.Accept("*/*")
	req.Agent(useragent)
	resp, err := req.Do()
	if err != nil {
		return sr.SimpleResponse{}, err
	}
	return resp, nil
}

func makeScreenshots(m []timemap) {

	log.Println("making screenshots")

	var count int

	//make archived screenshots...
	var err error
	for x := range m {
		if m[x].uri != "" && m[x].compliant && m[x].resolved != false && screenshot {
			m[x].screenshot, err = ps.GrabScreenshot(m[x].uri)
			if err != nil {
				log.Println("error creating screenshot for ", m[x].uri, err)
			} else {
				count++
				m[x].captured = true
			}
		}
	}

	log.Println(strconv.Itoa(count), "screenshots made")

}

func maketable(mementos []timemap) string {

	var tab string
	var noncompliant []string
	var others []string

	tab = tab + "<table><tr><th>uri</th><th>snapshot</th></tr>"

	for i := range mementos {

		/*
			id        string
			uri       string
			compliant bool
			original  string
			timegate  string
			screenshot string
		*/

		if mementos[i].compliant && mementos[i].resolved && mementos[i].captured != false {
			tab = tab + "<tr><td><a href='" + mementos[i].uri + "' target='_blank'>" + mementos[i].uri + "</a></td><td><img src='" + mementos[i].screenshot + "'/></td></tr>"
		} else if mementos[i].compliant && mementos[i].resolved && mementos[i].captured == false {
			tab = tab + "<tr><td><a href='" + mementos[i].uri + "' target='_blank'>" + mementos[i].uri + "</a></td><td><b>not captured</b></td></tr>"
		} else {
			noncompliant = append(noncompliant, mementos[i].id)
		}

		if mementos[i].compliant && mementos[i].resolved == false {
			others = append(others, mementos[i].id)
		}
	}

	tab = tab + "</table>"

	var noncomp = "<b>Noncompliant Timegates</b><pre>"
	for i := range noncompliant {
		noncomp = noncomp + noncompliant[i] + "<br/>"
	}
	noncomp = noncomp + "</pre>"

	var oth = "<b>Compliant but no resource</b><pre>"
	for i := range others {
		oth = oth + others[i] + "<br/>"
	}
	oth = oth + "</pre>"

	return noncomp + "<br/>" + oth + "<br/><b>Good Timegates</b><br/>" + tab
}

func timegate(url string, date string) string {
	//1. maketimemap for baseuri
	//2. extract from timemap
	//3, for compliant urls get next date for form date
	//4. with a new uri take screenshot
	//5. display, in table...

	//get timemap data...
	m, _ := getTimemap(makeTimemap(url))

	//get the date of the snapshot in the archive closest to that we requested...
	addDateURIs(m, url, date)

	//make screenshots
	makeScreenshots(m)

	return maketable(m)
}

func main() {

	var wg sync.WaitGroup
	wg.Add(1)
	go DefaultServer(port, &wg)

	log.Println("Server started on:", port)
	log.Println("Using:", sr.Version())

	wg.Wait()
	return
}
