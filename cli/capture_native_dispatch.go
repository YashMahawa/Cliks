package main

import "sync"

// nativeCaptureDispatcher keeps platform hook callbacks short without dropping
// human input bursts. OS callbacks only append to this in-memory queue; one
// worker performs the cancellation-aware send into ActivityCapture.Events.
type nativeCaptureDispatcher struct {
	mu      sync.Mutex
	ready   *sync.Cond
	queue   []LocalActivityEvent
	stopped bool
	done    chan struct{}
}

func newNativeCaptureDispatcher(capture *ActivityCapture) *nativeCaptureDispatcher {
	d := &nativeCaptureDispatcher{done: make(chan struct{})}
	d.ready = sync.NewCond(&d.mu)
	go func() {
		defer close(d.done)
		for {
			d.mu.Lock()
			for len(d.queue) == 0 && !d.stopped {
				d.ready.Wait()
			}
			if len(d.queue) == 0 && d.stopped {
				d.mu.Unlock()
				return
			}
			event := d.queue[0]
			d.queue[0] = LocalActivityEvent{}
			d.queue = d.queue[1:]
			d.mu.Unlock()
			capture.emit(event)
		}
	}()
	return d
}

func (d *nativeCaptureDispatcher) push(event LocalActivityEvent) {
	d.mu.Lock()
	if !d.stopped {
		d.queue = append(d.queue, event)
		d.ready.Signal()
	}
	d.mu.Unlock()
}

func (d *nativeCaptureDispatcher) stop() {
	d.mu.Lock()
	d.stopped = true
	d.queue = nil
	d.ready.Broadcast()
	d.mu.Unlock()
	<-d.done
}
