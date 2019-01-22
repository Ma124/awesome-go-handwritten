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

	"github.com/gorilla/mux"
	gfm "github.com/shurcooL/github_flavored_markdown"
	"github.com/xeonx/timeago"
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

type content struct {
	Body string
}

type ghJson struct {
	Stars      int    `json:"stargazers_count"`
	Forks      int    `json:"forks_count"`
	Issues     int    `json:"open_issues_count"`
	LastCommit string `json:"pushed_at"`
	Message    string `json:"message"`
}

var githubToken = ""
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

	f, err := os.Open(readmePath)
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
			// stargazers_count forks_count pushed_at open_issues_count
			afterNameI := strings.IndexByte(l, ']')
			name := l[starI+3 : afterNameI]
			desc := l[strings.Index(l, " - ")+3 : len(l)-1]
			url := l[afterNameI+2 : strings.IndexByte(l[afterNameI+2:], ')')+afterNameI+2]
			if strings.HasPrefix(url, "https://github.com/") {
				var data []byte
				reqCounts++
				if reqCounts < maxReqCount {
					resp, err := http.Get("https://api.github.com/repos/" + url[19:] + "?access_token=" + githubToken)
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
				err = json.Unmarshal(data, jsO)
				if err != nil {
					panic(err)
				}

				if jsO.Message != "" {
					WriteTableColumns(buf, "N/A", "N/A", "N/A", "N/A", name, url, desc)
				} else {
					t, err := time.Parse(time.RFC3339, jsO.LastCommit)
					if err != nil {
						panic(err)
					}

					WriteTableColumns(buf, strconv.Itoa(jsO.Stars), strconv.Itoa(jsO.Forks), strconv.Itoa(jsO.Issues), timeago.English.Format(t), name, url, desc)
				}
			} else {
				WriteTableColumns(buf, "N/A", "N/A", "N/A", "N/A", name, url, desc)
			}
		} else {
			writeTblHead = true
			buf.WriteString(l)
		}
	}
	return buf.String()
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

func hookHandler(w http.ResponseWriter, r *http.Request) {
	go generateHTML()
	w.Write(doneResp)
}

func main() {
	compile := false
	flag.BoolVar(&compile, "compile", false, "Don't launch a server and just compile it once")
	flag.StringVar(&githubToken, "gh-token", "", "Generate one at https://github.com/settings/tokens")
	flag.IntVar(&maxReqCount, "max-req-count", maxInt, "Only for development.")
	flag.BoolVar(&noCheckout, "no-checkout", false, "Don't run `git checkout force`")

	flag.Parse()

	if githubToken == "" {
		panic("You need to specify a --gh-token")
	}

	if compile {
		generateHTML()
		return
	} else {
		r := mux.NewRouter()
		r.HandleFunc("/hook", hookHandler)
		http.ListenAndServe(":9000", r)
	}
}
