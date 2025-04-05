package core

// --- IP Blocking Middleware Function ---

// BlockMiddleware creates a middleware function that uses a ConcurrentSketch
// to identify and potentially block IPs based on request frequency.
//func (a *App) BlockMiddleware() func(http.Handler) http.Handler {
//	// TODO
//	// Initialize the underlying sketch
//	sketch := sliding.New(3, 10, sliding.WithWidth(1024), sliding.WithDepth(3))
//	a.Logger().Info("sketch memory usage", "bytes", sketch.SizeBytes())
//
//	// Create a new ConcurrentSketch with default tick size
//	cs := NewConcurrentSketch(sketch, 100) // Default tickSize
//
//	// Return the middleware function
//	return func(next http.Handler) http.Handler {
//		fn := func(w http.ResponseWriter, r *http.Request) {
//			ip := a.GetClientIP(r)
//
//			// Check if IP is already blocked
//
//			// TODO not here
//			if a.IsBlocked(ip) {
//				writeJsonError(w, errorIpBlocked)
//				a.Logger().Info("IP blocked from accessing endpoint", "ip", ip)
//				return
//			}
//
//			blockedIPs := cs.processTick(ip)
//
//			// Handle blocking outside the mutex
//			//
//			// Even if multiple goroutines call a.BlockIP for the same IP
//			// concurrently, Ristretto will handle it safely. Blocking an IP
//			// multiple times is harmless if the operation is idempotent (same key).
//			// Ristretto batches writes into a ring buffer, so frequent Set calls
//			// for the same key will be merged efficiently. The last write (in
//			// buffer order) will determine the final value.
//			// Ristretto uses a buffered write mechanism (a ring buffer) to batch
//			// Set/Del operations for performance.
//			if len(blockedIPs) > 0 {
//				a.Logger().Info("IPs to be blocked", "ips", blockedIPs)
//				go func(ips []string) {
//					for _, ip := range ips {
//						if err := a.BlockIP(ip); err != nil {
//							a.Logger().Error("failed to block IP", "ip", ip, "error", err)
//						}
//					}
//				}(blockedIPs)
//			}
//
//			// Proceed to the next handler in the chain
//			next.ServeHTTP(w, r)
//		}
//		return http.HandlerFunc(fn)
//	}
//}
