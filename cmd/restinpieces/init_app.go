package main

//import (
//	"log/slog"
//	"os"
//
//	"github.com/caasmo/restinpieces/core"
//	// TOD0 problem cgo compile check?
//	"github.com/caasmo/restinpieces/cache/ristretto"
//	"github.com/caasmo/restinpieces/config"
//	"github.com/caasmo/restinpieces/db/crawshaw"
//	"github.com/caasmo/restinpieces/db/zombiezen"
//	"github.com/caasmo/restinpieces/router/httprouter"
//	"github.com/caasmo/restinpieces/router/servemux"
//	phuslog "github.com/phuslu/log"
//)
//
//
//func initApp(cfg *config.Config) (*core.App, error) {
//
//	return core.NewApp(
//		WithDBCrawshaw(cfg.DBFile),
//		WithRouterServeMux(),
//		WithCacheRistretto(),
//		core.WithConfig(cfg),
//		//WithPhusLogger(nil), // Provide the logger using defaults
//		WithTextLogger(nil), // Provide the logger using defaults
//	)
//}
