FROM golang:1.10-alpine

RUN apk add --update -t build-deps curl go git libc-dev gcc libgcc
RUN go get -u -v github.com/shurcooL/github_flavored_markdown github.com/gorilla/mux github.com/xeonx/timeago

WORKDIR /srv

CMD ["go", "run", "repo.go", "--gh-token", "$GH_TOKEN"]
