package cache

// Incomplete interface
type Cache interface {
	Get(interface{}) (interface{}, bool)
	Set(interface{}, interface{}, int64) bool
}
