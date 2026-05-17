package engine

// Notifier receives deal alerts. Implementations are fire-and-forget;
// they must not block the poller goroutine.
type Notifier interface {
	Notify(Deal)
}
