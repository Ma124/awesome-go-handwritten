package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/template"
	"time"

	"fmt"
	"github.com/gorilla/mux"
	gfm "github.com/shurcooL/github_flavored_markdown"
	"github.com/xeonx/timeago"
	"gitlab.com/Ma_124/progressbar"
)

// memory usage optimizations
const (
	emtyStr  = ""
	maxInt   = int((^uint(0)) >> 1)
	git      = "git"
	checkout = "checkout"
	force    = "-f"
	pull     = "pull"

	// options
	readmePath = "./README.md"
	tplPath    = "tmpl/tmpl.html"
	idxPath    = "tmpl/index.html"
)

var (
	doneResp = []byte("Done!\n")
)

var (
	f    *os.File
	size int64
	pb   *progressbar.ProgressBar
)

type content struct {
	Body string
}

var githubToken = ""
var gitlabToken = ""
var maxReqCount = 0
var noCheckout = false

func generateHTML() {
	// Update repo
	if !noCheckout {
		_, err := exec.Command(git, checkout, force).Output()
		if err != nil {
			panic(err)
		}

		_, err = exec.Command(git, pull).Output()
		if err != nil {
			panic(err)
		}
	}

	fi, err := os.Stat(readmePath)
	if err != nil {
		panic(err)
	}
	size = fi.Size()
	pb = progressbar.New(int(size))

	f, err = os.Open(readmePath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	body := string(gfm.Markdown([]byte(processReadme(bufio.NewReader(f)))))
	c := &content{Body: body}

	t := template.Must(template.ParseFiles(tplPath))
	f2, err := os.Create(idxPath)
	if err != nil {
		panic(err)
	}
	defer f2.Close()

	err = t.Execute(f2, c)
	if err != nil {
		panic(err)
	}
}

func processReadme(r *bufio.Reader) string {
	buf := &strings.Builder{}
	writeTblHead := true
	i := 0
	reqCounts := 0
	for {
		printProgressBar()
		l, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}

		i++

		starI := strings.IndexByte(l, '*')

		if starI != -1 && (l[0] == ' ' || l[0] == '*') && strings.HasPrefix(l[starI:], "* [") {
			if writeTblHead {
				writeTblHead = false
				buf.WriteString("\n| Stars | Forks | Open Issues | Last Commit | Name | Description |\n")
				buf.WriteString("|-------|-------|-------------|-------------|------|-------------|\n")
			}
			afterNameI := strings.IndexByte(l, ']')
			name := l[starI+3 : afterNameI]
			desc := l[strings.Index(l, " - ")+3 : len(l)-1]
			url := l[afterNameI+2 : strings.IndexByte(l[afterNameI+2:], ')')+afterNameI+2]
			if strings.HasPrefix(url, "https://github.com/") {
				reqCounts = fetchAndWriteGitHub(buf, name, url, desc, reqCounts, url[19:])
			} else if strings.HasPrefix(url, "https://godoc.org/github.com/") {
				reqCounts = fetchAndWriteGitHub(buf, name, url, desc, reqCounts, url[29:])
			} else if strings.HasPrefix(url, "http://github.com/") {
				reqCounts = fetchAndWriteGitHub(buf, name, url, desc, reqCounts, url[18:])
			} else if strings.HasPrefix(url, "http://godoc.org/github.com/") {
				reqCounts = fetchAndWriteGitHub(buf, name, url, desc, reqCounts, url[28:])
			} else if strings.HasPrefix(url, "https://gitlab.com/") {
				reqCounts = fetchAndWriteGitLab(buf, name, url, desc, reqCounts, strings.Replace(url[19:], "/", "%2F", -1))
			} else if strings.HasPrefix(url, "http://gitlab.com/") {
				reqCounts = fetchAndWriteGitLab(buf, name, url, desc, reqCounts, strings.Replace(url[18:], "/", "%2F", -1))
			} else {
				dotI := strings.IndexByte(url, '.')
				postDot := url[dotI+1:]

				if strings.HasPrefix(postDot, "github.io") {
					slashI := strings.IndexByte(postDot, '/')
					if slashI > 0 {
						if url[7] == '/' {
							reqCounts = fetchAndWriteGitHub(buf, name, url, desc, reqCounts, url[8:dotI]+"/"+postDot[slashI+1:])
						} else {
							reqCounts = fetchAndWriteGitHub(buf, name, url, desc, reqCounts, url[7:dotI]+"/"+postDot[slashI+1:])
						}
					} else {
						WriteNATableColumns(buf, name, url, desc)
					}
				} else {
					WriteNATableColumns(buf, name, url, desc)
				}
			}
		} else {
			writeTblHead = true
			buf.WriteString(l)
		}
	}

	pb.Finish()
	return buf.String()
}

func printProgressBar() {
	offset, err := f.Seek(0, 1)
	if err != nil {
		fmt.Println("cannot seek current position")
	}

	pb.Set(int(offset))
}

type ghJson struct {
	Stars      int    `json:"stargazers_count"`
	Forks      int    `json:"forks_count"`
	Issues     int    `json:"open_issues_count"`
	LastCommit string `json:"pushed_at"`
	Message    string `json:"message"`
}

func fetchAndWriteGitHub(buf *strings.Builder, name, url, desc string, reqCounts int, apiUrl string) (reqCountz int) {
	var data []byte

	if url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}

	if reqCounts < maxReqCount {
		resp, err := http.Get("https://api.github.com/repos/" + apiUrl + "?access_token=" + githubToken)
		if err != nil {
			panic(err)
		}

		data, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		resp.Body.Close()
	} else {
		data = []byte(`{ "stargazers_count": 1234, "forks_count": 56, "open_issues_count": 7, "pushed_at": "2019-01-20T19:24:24Z" }`)
	}

	jsO := &ghJson{}
	err := json.Unmarshal(data, jsO)
	if err != nil {
		panic(err)
	}

	if jsO.Message != "" {
		WriteNATableColumns(buf, name, url, desc)
	} else {
		t, err := time.Parse(time.RFC3339, jsO.LastCommit)
		if err != nil {
			panic(err)
		}

		WriteTableColumns(buf, strconv.Itoa(jsO.Stars), strconv.Itoa(jsO.Forks), strconv.Itoa(jsO.Issues), timeago.English.Format(t), name, url, desc) // TODO fork and improve timeago
	}

	return reqCounts + 1
}

type glJson struct {
	Stars      int    `json:"star_count"`
	Forks      int    `json:"forks_count"`
	Issues     int    `json:""` // len(.../issues)
	LastCommit string `json:"last_activity_at"`
	Message    string `json:"message"`
}

func fetchAndWriteGitLab(buf *strings.Builder, name, url, desc string, reqCounts int, apiUrl string) (reqCountz int) {
	var data []byte

	if url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}

	if reqCounts < maxReqCount {
		resp, err := http.Get("https://gitlab.com/api/v4/projects/" + apiUrl + "?access_token=" + gitlabToken)
		if err != nil {
			panic(err)
		}

		data, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		resp.Body.Close()
	} else {
		data = []byte(`{ "star_count": 1234, "forks_count": 56, "": 7, "last_activity_at": "2019-01-20T19:24:24Z" }`)
	}

	jsO := &glJson{}
	err := json.Unmarshal(data, jsO)
	if err != nil {
		panic(err)
	}

	if jsO.Message != "" {
		WriteNATableColumns(buf, name, url, desc)
	} else {
		t, err := time.Parse(time.RFC3339, jsO.LastCommit)
		if err != nil {
			panic(err)
		}

		WriteTableColumns(buf, strconv.Itoa(jsO.Stars), strconv.Itoa(jsO.Forks), "N/A", timeago.English.Format(t), name, url, desc) // TODO fork, improve timeago and issues
	}

	return reqCounts + 1
}

func WriteTableColumns(buf *strings.Builder, stars, forks, openIssues, lastCommit, name, url, desc string) {
	buf.WriteString("| ")
	buf.WriteString(stars)
	buf.WriteString(" | ")
	buf.WriteString(forks)
	buf.WriteString(" | ")
	buf.WriteString(openIssues)
	buf.WriteString(" | ")
	buf.WriteString(lastCommit)
	buf.WriteString(" | [")
	buf.WriteString(name)
	buf.WriteString("](")
	buf.WriteString(url)
	buf.WriteString(") | ")
	buf.WriteString(desc)
	buf.WriteString(" |\n")
}

func WriteNATableColumns(buf *strings.Builder, name, url, desc string) {
	WriteTableColumns(buf, "N/A", "N/A", "N/A", "N/A", name, url, desc)
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	go generateHTML()
	w.Write(doneResp)
}

func main() {
	compile := false
	flag.BoolVar(&compile, "compile", false, "Don't launch a server and just compile it once")
	flag.StringVar(&githubToken, "gh-token", "", "Generate one at https://github.com/settings/tokens")
	flag.StringVar(&gitlabToken, "gl-token", "", "Generate one at https://gitlab.com/profile/applications")
	flag.IntVar(&maxReqCount, "max-req-count", maxInt, "Only for development.")
	flag.BoolVar(&noCheckout, "no-checkout", false, "Don't run `git checkout force`")

	flag.Parse()

	if githubToken == "" {
		panic("You need to specify a --gh-token")
	}

	if compile {
		println("Compiling HTML")
		generateHTML()
		return
	} else {
		println("Starting server")
		r := mux.NewRouter()
		r.HandleFunc("/hook", hookHandler)
		http.ListenAndServe(":9000", r)
	}
}
