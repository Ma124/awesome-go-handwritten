package main

import (
	"bufio"
	"encoding/json"
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
}

func generateHTML() {
	// Update repo
	_, err := exec.Command(git, checkout, force).Output()
	if err != nil {
		panic(err)
	}

	_, err = exec.Command(git, pull).Output()
	if err != nil {
		panic(err)
	}

	f, err := os.Open(readmePath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	input := bufio.NewReader(f)
	body := string(gfm.Markdown([]byte(processReadme(input))))
	c := &content{Body: body}

	t := template.Must(template.ParseFiles(tplPath))
	f2, err := os.Create(idxPath)
	if err != nil {
		panic(err)
	}
	defer f2.Close()

	err = t.Execute(f, c)
	if err != nil {
		panic(err)
	}
}

func processReadme(r *bufio.Reader) string {
	buf := strings.Builder{}
	writeTblHead := true
	i := 0
	for {
		l, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}

		println(i)
		i++

		if !(l[0] == '*' && l[1] != ' ') {
			writeTblHead = true
			buf.WriteString(l)
		} else {
			if writeTblHead {
				buf.WriteString("| Stars | Forks | Open Issues | Last Commit | Name | Description |\n")
				buf.WriteString("|-------|-------|-------------|-------------|------|-------------|\n")
				// stargazers_count forks_count pushed_at open_issues_count
				afterNameI := strings.LastIndexByte(l, ']')
				name := l[4:afterNameI]
				println(name)
				desc := l[strings.Index(l, " - ")+1:len(l)-1]
				url := l[afterNameI+2 : strings.LastIndexByte(l, ')')]
				if strings.HasPrefix(l, "https://github.com/") {
					url = url[20:]
					resp, err := http.Get("https://api.github.com/repos/" + url)
					if err != nil {
						panic(err)
					}

					data, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						panic(err)
					}
					resp.Body.Close()

					jsO := &ghJson{}
					err = json.Unmarshal(data, jsO)
					if err != nil {
						panic(err)
					}

					buf.WriteString("| ")
					buf.WriteString(strconv.Itoa(jsO.Stars))
					buf.WriteString(":star: | ")
					buf.WriteString(strconv.Itoa(jsO.Forks))
					buf.WriteString(" | ")
					buf.WriteString(strconv.Itoa(jsO.Issues))
					buf.WriteString(" | ")

					t, err := time.Parse(time.RFC3339, jsO.LastCommit)
					if err != nil {
						panic(err)
					}
					buf.WriteString(timeago.English.Format(t))

					buf.WriteString(name)
					buf.WriteString(" | ")
					buf.WriteString(desc)
					buf.WriteString(" |")
				} else {
					buf.WriteString("| N/A | N/A | N/A | N/A | ")
					buf.WriteString(name)
					buf.WriteString(" | ")
					buf.WriteString(desc)
					buf.WriteString(" |")
				}
			}
		}
	}
	return buf.String()
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	go generateHTML()
	w.Write(doneResp)
}

func main() {
	if len(os.Args) == 2 {
		if os.Args[1] == "compile" {
			generateHTML()
			return
		} else if os.Args[1] == "hook-server" {
		} else {
			println("Unknown command")
			return
		}
	}
	r := mux.NewRouter()
	r.HandleFunc("/hook", hookHandler)
	http.ListenAndServe(":9000", r)
}
