package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint"
)

func main() {
	ctx := context.Background()
	root := entrypoint.NewRootCommand()
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "mybox:", err)
		if errors.Is(err, domain.ErrInvalidArgument) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
