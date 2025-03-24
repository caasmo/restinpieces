package custom

import "github.com/caasmo/restinpieces/core"

type App struct {
	*app.App // Embedding app.App
}

func NewApp(ap *app.App) *App {
	return &App{
		App: ap,
	}
}
