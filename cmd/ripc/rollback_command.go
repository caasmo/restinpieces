package main

import (
	"fmt"
	"os"

	"github.com/caasmo/restinpieces/config"
)

func handleRollbackCommand(secureStore config.SecureStore, scope string, generation int) {
	if scope == "" {
		scope = config.ScopeApplication
	}

	if generation < 1 {
		fmt.Fprintf(os.Stderr, "Error: can only rollback to generation 1 or higher\n")
		os.Exit(1)
	}

	// Get the target generation config
	targetData, format, err := secureStore.Get(scope, generation)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get config generation %d: %v\n", generation, err)
		os.Exit(1)
	}

	// Save it as the new latest version
	err = secureStore.Save(scope, targetData, format, fmt.Sprintf("Rollback to generation %d", generation))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save rollback config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully rolled back scope '%s' to generation %d\n", scope, generation)
}
