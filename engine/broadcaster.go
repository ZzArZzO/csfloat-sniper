package engine

import "sync"

type Broadcaster struct {
	mu      sync.Mutex
	clients map[chan Deal]struct{}
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{clients: make(map[chan Deal]struct{})}
}

func (b *Broadcaster) Subscribe() chan Deal {
	ch := make(chan Deal, 16)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broadcaster) Unsubscribe(ch chan Deal) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *Broadcaster) Publish(deal Deal) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- deal:
		default:
		}
	}
}
