package custom

import "github.com/caasmo/restinpieces/core"

type App struct {
	*core.App // Embedding app.App
}

func NewApp(ap *core.App) *App {
	return &App{
		App: ap,
	}
}
