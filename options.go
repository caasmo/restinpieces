package restinpieces

// this shoudl be under custom???
// just initilizes the custom packges that implments the app
import (
	"fmt"

	"github.com/caasmo/restinpieces/cache/ristretto"
	"github.com/caasmo/restinpieces/core"
)


func WithCacheRistretto() core.Option {
	cacheInstance, err := ristretto.New[any]() // Explicit string keys and interface{} values
	if err != nil {
		panic(fmt.Sprintf("failed to initialize ristretto cache: %v", err))
	}
	return core.WithCache(cacheInstance)
}
