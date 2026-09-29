// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	// queueCapacity bounds the number of objects waiting for a worker. When
	// the queue is full the translations stay in the processing state and the
	// recovery sweep picks them up later.
	queueCapacity = 10000
	// workerStopTimeout bounds how long Shutdown waits for in-flight tasks.
	workerStopTimeout = 15 * time.Second
)

var (
	errWorkerStopped = errors.New("translation workers are stopped")
	errQueueFull     = errors.New("translation queue is full")
)

// task asks the workers to translate one object into a set of languages.
type task struct {
	objectType string
	objectID   string
	channelID  string
	content    *sourceContent
	langs      []string
	// srcLang is the source language when already known (local detection).
	srcLang string
	srcConf *float64
	// locales maps each base language to the full user locales it serves, so
	// websocket events can be keyed the way every client looks them up.
	locales map[string][]string
}

func (t *task) objectKey() string {
	return t.objectType + ":" + t.objectID + ":" + t.content.NormHash
}

// workerPool runs translation tasks on a fixed number of goroutines.
type workerPool struct {
	process  func(*task)
	onDepth  func(int)
	queue    chan *task
	mu       sync.Mutex
	running  bool
	inflight map[string]map[string]bool // object key -> languages queued or in progress
	stops    []chan struct{}
	wg       sync.WaitGroup
}

func newWorkerPool(process func(*task), onDepth func(int)) *workerPool {
	return &workerPool{
		process:  process,
		onDepth:  onDepth,
		queue:    make(chan *task, queueCapacity),
		inflight: make(map[string]map[string]bool),
	}
}

// start launches n workers. It is a no-op when the pool already runs.
func (p *workerPool) start(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return
	}
	p.running = true
	p.spawnLocked(n)
}

func (p *workerPool) spawnLocked(n int) {
	if n < 1 {
		n = 1
	}
	for range n {
		stop := make(chan struct{})
		p.stops = append(p.stops, stop)
		p.wg.Add(1)
		go p.run(stop)
	}
}

// resize changes the number of running workers.
func (p *workerPool) resize(n int) {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	stops := p.stops
	p.stops = nil
	p.mu.Unlock()

	for _, s := range stops {
		close(s)
	}
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		p.spawnLocked(n)
	}
}

// stop signals the workers and waits for them to finish their current task.
// Queued tasks are kept, still marked processing in the database, and are
// resumed by the recovery sweep.
func (p *workerPool) stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	stops := p.stops
	p.stops = nil
	p.mu.Unlock()

	for _, s := range stops {
		close(s)
	}

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(workerStopTimeout):
	}
}

func (p *workerPool) run(stop chan struct{}) {
	defer p.wg.Done()
	for {
		// Give the stop signal priority over pending work.
		select {
		case <-stop:
			return
		default:
		}
		select {
		case <-stop:
			return
		case t := <-p.queue:
			p.depth()
			p.process(t)
		}
	}
}

func (p *workerPool) depth() {
	if p.onDepth != nil {
		p.onDepth(len(p.queue))
	}
}

// enqueue schedules t. Languages already queued or in progress for the same
// object content are dropped from it; nothing is scheduled when none remain.
func (p *workerPool) enqueue(t *task) error {
	if t == nil {
		return errors.New("nil task")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return errWorkerStopped
	}

	key := t.objectKey()
	pending := p.inflight[key]
	var langs []string
	for _, l := range t.langs {
		if !pending[l] {
			langs = append(langs, l)
		}
	}
	if len(langs) == 0 {
		return nil
	}
	sort.Strings(langs)
	t.langs = langs

	select {
	case p.queue <- t:
	default:
		return errQueueFull
	}

	if pending == nil {
		pending = make(map[string]bool, len(langs))
		p.inflight[key] = pending
	}
	for _, l := range langs {
		pending[l] = true
	}
	p.depth()
	return nil
}

// done releases lang of the object t describes.
func (p *workerPool) done(t *task, lang string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := t.objectKey()
	if pending := p.inflight[key]; pending != nil {
		delete(pending, lang)
		if len(pending) == 0 {
			delete(p.inflight, key)
		}
	}
}

// isInflight reports whether any language of the object is queued or being
// translated, whatever its content hash.
func (p *workerPool) isInflight(objectType, objectID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	prefix := objectType + ":" + objectID + ":"
	for key := range p.inflight {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// workerCount returns the configured number of workers.
func workerCount(cfg *model.Config) int {
	n := model.SafeDereference(cfg.AutoTranslationSettings.Workers)
	if n < 1 {
		n = model.AutoTranslationDefaultWorkers
	}
	if n > 64 {
		n = 64
	}
	return n
}
