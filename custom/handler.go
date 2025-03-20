package custom

import (
	"fmt"
	"net/http"
)

func (a *App) Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to your handlers!")
}
