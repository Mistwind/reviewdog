
all: build

build:
	go build github.com/reviewdog/reviewdog/cmd/reviewdog
clean:
	rm -f reviewdog

install: 
	cp reviewdog /usr/local/bin
