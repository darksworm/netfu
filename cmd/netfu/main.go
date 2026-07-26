package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend"
	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/backend/nm"
	"github.com/ilmars/netfu/internal/tui"
)

// version is overridable at build time:
// go build -ldflags "-X main.version=v1.2.3" ./cmd/netfu
var version = "dev"

func main() {
	useFake := flag.Bool("fake", false, "run against seeded fixture data instead of NetworkManager")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("netfu", version)
		return
	}

	var b backend.Backend
	if *useFake {
		b = fake.SeedArchLaptop()
	} else {
		adapter, err := nm.New()
		if err != nil {
			fmt.Fprintln(os.Stderr, "netfu:", err)
			os.Exit(1)
		}
		b = adapter
	}

	p := tea.NewProgram(tui.New(b))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "netfu:", err)
		os.Exit(1)
	}
}
