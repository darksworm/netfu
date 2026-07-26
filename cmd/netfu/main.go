package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/ilmars/netfu/internal/backend/fake"
	"github.com/ilmars/netfu/internal/tui"
)

func main() {
	// The NetworkManager adapter lands in a later milestone; until then the
	// binary runs against seeded fixture data so the shell stays usable.
	b := fake.SeedArchLaptop()
	p := tea.NewProgram(tui.New(b))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "netfu:", err)
		os.Exit(1)
	}
}
