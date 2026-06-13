// Package watcher provides a simple file-system watcher that monitors
// a root directory for WASM/skill changes and triggers a reload callback.
//
// Usage:
//
//	w, err := watcher.NewWatcher(skillsDir, func(ctx context.Context, skillName string) error {
//	    fmt.Println("reloading skill:", skillName)
//	    return nil
//	})
//	w.Start(ctx)
//	defer w.Stop()
package watcher

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ReloadCallback is called when a file change is detected.
// skillName is the top-level directory name inside the watch root.
type ReloadCallback func(ctx context.Context, skillName string) error

// Watcher monitors files and triggers reloads with debounce.
type Watcher struct {
	root     string
	cb       ReloadCallback
	watcher  *fsnotify.Watcher
	ctx      context.Context
	cancel   context.CancelFunc
	debounce time.Duration
	pending  map[string]*time.Timer
	mu       sync.Mutex
	running  bool
}

// NewWatcher creates a new watcher rooted at rootDir.
func NewWatcher(rootDir string, cb ReloadCallback) *Watcher {
	return &Watcher{
		root:     rootDir,
		cb:       cb,
		debounce: 300 * time.Millisecond,
		pending:  make(map[string]*time.Timer),
	}
}

// Start begins watching. Returns an error if the watcher cannot be created
// or the root directory does not exist.
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("watcher already running")
	}
	w.running = true
	w.mu.Unlock()

	var err error
	w.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	w.ctx, w.cancel = context.WithCancel(ctx)

	if err := w.watchRecursive(w.root); err != nil {
		_ = w.watcher.Close()
		return fmt.Errorf("watch root: %w", err)
	}

	go w.loop()
	log.Printf("[watcher] watching %s", w.root)
	return nil
}

// Stop shuts down the watcher. Safe to call multiple times.
func (w *Watcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return nil
	}
	w.running = false
	if w.cancel != nil {
		w.cancel()
	}
	for _, t := range w.pending {
		t.Stop()
	}
	w.pending = nil
	if w.watcher != nil {
		return w.watcher.Close()
	}
	return nil
}

// IsRunning reports whether the watcher is active.
func (w *Watcher) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// --- internal ---

func (w *Watcher) watchRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip inaccessible paths; continue scanning.
			return nil
		}
		if info.IsDir() {
			if e := w.watcher.Add(path); e != nil {
				log.Printf("[watcher] warn: cannot add %s: %v", path, e)
			}
		}
		return nil
	})
}

func (w *Watcher) loop() {
	defer func() {
		if w.watcher != nil {
			_ = w.watcher.Close()
		}
	}()

	for {
		select {
		case <-w.ctx.Done():
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[watcher] error: %v", err)
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Track newly created directories so they are watched too.
	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			_ = w.watcher.Add(event.Name)
		}
	}
	// Only react to writes or creates of relevant files.
	if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
		return
	}
	ext := strings.ToLower(filepath.Ext(event.Name))
	base := filepath.Base(event.Name)
	if ext != ".wasm" && ext != ".soi" && base != "skill.yaml" && base != "skill.yml" {
		return
	}

	rel, err := filepath.Rel(w.root, event.Name)
	if err != nil {
		return
	}
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	if len(parts) == 0 {
		return
	}
	skillName := parts[0]

	w.mu.Lock()
	if old, ok := w.pending[skillName]; ok {
		old.Stop()
	}
	w.pending[skillName] = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		delete(w.pending, skillName)
		w.mu.Unlock()

		log.Printf("[watcher] reloading skill %q", skillName)
		if err := w.cb(context.Background(), skillName); err != nil {
			log.Printf("[watcher] reload %q failed: %v", skillName, err)
		}
	})
	w.mu.Unlock()
}
