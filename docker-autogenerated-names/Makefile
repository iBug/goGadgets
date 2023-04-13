main: main.go names-generator.go
	go build -ldflags='-s -w' -o $@ $^

names-generator.go:
	wget -O $@ https://raw.githubusercontent.com/moby/moby/master/pkg/namesgenerator/names-generator.go
	sed -i '/^package /c\package main' $@
