package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/ryanuber/columnize"
)

var (
	ls = strings.Join(left[:], "|")
	rs = strings.Join(right[:], "|")
	re = regexp.MustCompile(fmt.Sprintf("\\b(%s)_(%s)\\b", ls, rs))
)

func isAutoName(s string) bool {
	return re.FindString(s) != ""
}

func main() {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		panic(err)
	}

	options := types.ContainerListOptions{
		All: true,
	}
	containers, err := cli.ContainerList(ctx, options)
	if err != nil {
		panic(err)
	}

	lines := []string{"X | State | ID | Name | Image"}
	for _, c := range containers {
		name := strings.TrimPrefix(c.Names[0], "/")
		line := []string{".",
			c.State,
			c.ID[:10],
			name,
			c.Image,
		}
		if isAutoName(name) {
			line[0] = "X"
		}
		lines = append(lines, strings.Join(line, " | "))
	}
	fmt.Println(columnize.SimpleFormat(lines))
}
