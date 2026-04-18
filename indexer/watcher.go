package indexer

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileEvent represents a file change event.
type FileEvent struct {
	Path string
	Op   fsnotify.Op
}

// Watcher watches directories for file changes with debouncing.
type Watcher struct {
	watcher  *fsnotify.Watcher
	filter   *Filter
	events   chan FileEvent
	done     chan struct{}
	debounce time.Duration

	mu      sync.Mutex
	pending map[string]*time.Timer
}

// NewWatcher creates a Watcher with the given filter and debounce duration.
func NewWatcher(filter *Filter, debounce time.Duration) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if debounce == 0 {
		debounce = 500 * time.Millisecond
	}

	w := &Watcher{
		watcher:  fw,
		filter:   filter,
		events:   make(chan FileEvent, 100),
		done:     make(chan struct{}),
		debounce: debounce,
		pending:  make(map[string]*time.Timer),
	}

	go w.loop()
	return w, nil
}

// Events returns the channel of debounced file events.
func (w *Watcher) Events() <-chan FileEvent {
	return w.events
}

// AddRecursive adds a directory and all its subdirectories to the watch list.
func (w *Watcher) AddRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if !w.filter.AcceptDir(path) {
				return filepath.SkipDir
			}
			return w.watcher.Add(path)
		}
		return nil
	})
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	close(w.done)

	w.mu.Lock()
	for _, t := range w.pending {
		t.Stop()
	}
	w.pending = nil
	w.mu.Unlock()

	return w.watcher.Close()
}

func (w *Watcher) loop() {
	for {
		select {
		case <-w.done:
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case _, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			// Errors are logged but don't stop the watcher.
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	// For new directories, start watching them.
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			if w.filter.AcceptDir(path) {
				w.AddRecursive(path)
			}
			return
		}
	}

	// Only process file events that pass the filter.
	if !w.filter.Accept(path) {
		return
	}

	// Debounce: reset timer for this path.
	w.mu.Lock()
	if t, exists := w.pending[path]; exists {
		t.Stop()
	}
	w.pending[path] = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		delete(w.pending, path)
		w.mu.Unlock()

		fe := FileEvent{Path: path, Op: event.Op}
		select {
		case w.events <- fe:
		case <-w.done:
		}
	})
	w.mu.Unlock()
}
