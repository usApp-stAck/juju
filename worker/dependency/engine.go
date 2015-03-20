package dependency

import (
	"time"

	"github.com/juju/errors"
	"github.com/juju/loggo"
	"launchpad.net/tomb"

	"github.com/juju/juju/worker"
)

var logger = loggo.GetLogger("juju.worker.dependency")

// workerInfo stores what an engine needs to know about the worker for a given
// Manifold.
type workerInfo struct {
	starting bool
	stopping bool
	worker   worker.Worker
}

// stopped returns true unless the worker is either assigned or starting.
func (info workerInfo) stopped() bool {
	switch {
	case info.worker != nil:
		return false
	case info.starting:
		return false
	}
	return true
}

// installTicket is used by engine to induce installation of a named manifold
// and pass on any errors encountered in the process.
type installTicket struct {
	name     string
	manifold Manifold
	result   chan<- error
}

// startedTicket is used by engine to notify the loop of the creation of a
// resource.
type startedTicket struct {
	name   string
	worker worker.Worker
}

// stoppedTicket is used by engine to notify the loop of the demise of (or
// failure to create) a resource.
type stoppedTicket struct {
	name  string
	error error
}

// engine maintains workers corresponding to its installed manifolds, and
// restarts them whenever their dependencies change.
type engine struct {
	tomb tomb.Tomb

	// isFatal allows errors generated by workers to stop the engine.
	isFatal func(error) bool

	// errorDelay controls how long the engine waits before restarting a worker
	// that encountered an unknown error.
	errorDelay time.Duration

	// bounceDelay controls how long the engine waits before restarting a worker
	// that was deliberately shut down because its dependencies changed.
	bounceDelay time.Duration

	// manifolds holds the installed manifolds by name.
	manifolds map[string]Manifold

	// dependents holds, for each named manifold, those that depend on it.
	dependents map[string][]string

	// current holds the active worker information for each installed manifold.
	current map[string]workerInfo

	install chan installTicket
	started chan startedTicket
	stopped chan stoppedTicket
}

// NewEngine returns an Engine that will maintain any Installed Manifolds until
// either the engine is killed or one of the manifolds' workers returns an error
// that satisfies isFatal.
func NewEngine(isFatal func(error) bool, errorDelay, bounceDelay time.Duration) Engine {
	engine := &engine{
		isFatal:     isFatal,
		errorDelay:  errorDelay,
		bounceDelay: bounceDelay,

		manifolds:  map[string]Manifold{},
		dependents: map[string][]string{},
		current:    map[string]workerInfo{},

		install: make(chan installTicket),
		started: make(chan startedTicket),
		stopped: make(chan stoppedTicket),
	}
	go func() {
		defer engine.tomb.Done()
		engine.tomb.Kill(engine.loop())
	}()
	return engine
}

func (engine *engine) loop() error {
	oneShotDying := engine.tomb.Dying()
	for {
		select {
		case <-oneShotDying:
			oneShotDying = nil
			for name := range engine.current {
				engine.stop(name)
			}
		case ticket := <-engine.install:
			// This is safe so long as the Install method reads the result.
			ticket.result <- engine.gotInstall(ticket.name, ticket.manifold)
		case ticket := <-engine.started:
			engine.gotStarted(ticket.name, ticket.worker)
		case ticket := <-engine.stopped:
			engine.gotStopped(ticket.name, ticket.error)
		}
		if engine.isDying() {
			if engine.allStopped() {
				return tomb.ErrDying
			}
		}
	}

}

// Kill is part of the worker.Worker interface.
func (engine *engine) Kill() {
	engine.tomb.Kill(nil)
}

// Wait is part of the worker.Worker interface.
func (engine *engine) Wait() error {
	return engine.tomb.Wait()
}

// Install is part of the Engine interface. It can be called by from any external
// goroutine.
func (engine *engine) Install(name string, manifold Manifold) error {
	result := make(chan error)
	select {
	case <-engine.tomb.Dying():
		return errors.New("engine is shutting down")
	case engine.install <- installTicket{name, manifold, result}:
		// This is safe so long as the loop sends a result.
		return <-result
	}
}

// gotInstall handles the params originally supplied to Install. It must only be
// called from the loop goroutine.
func (engine *engine) gotInstall(name string, manifold Manifold) error {
	logger.Infof("installing %s manifold...", name)
	if _, found := engine.manifolds[name]; found {
		return errors.Errorf("%s manifold already installed", name)
	}
	for _, input := range manifold.Inputs {
		if _, found := engine.manifolds[input]; !found {
			return errors.Errorf("%s manifold depends on unknown %s manifold", name, input)
		}
	}
	engine.manifolds[name] = manifold
	for _, input := range manifold.Inputs {
		engine.dependents[input] = append(engine.dependents[input], name)
	}
	engine.current[name] = workerInfo{}
	engine.start(name, 0)
	return nil
}

// start invokes a runWorker goroutine for the manifold with the supplied name.
func (engine *engine) start(name string, delay time.Duration) {

	// If we're shutting down, just don't do anything.
	if engine.isDying() {
		logger.Infof("not starting %s manifold worker (shutting down)", name)
		return
	}

	// Check preconditions.
	manifold, found := engine.manifolds[name]
	if !found {
		engine.tomb.Kill(errors.Errorf("fatal: unknown manifold %s", name))
	}

	// Copy current info and check more preconditions...
	info := engine.current[name]
	if !info.stopped() {
		engine.tomb.Kill(errors.New("fatal: trying to start a second %s manifold worker"))
	}

	// ...then update the info, copy it back to the engine, and start a worker
	// goroutine.
	info.starting = true
	engine.current[name] = info
	go engine.runWorker(name, manifold, delay)
}

// runWorker starts the supplied manifold's worker and communicates it back to the
// loop goroutine; waits for worker completion; and communicates any error encountered
// back to the loop goroutine. It's intended to be run on its own goroutine, but
// should only be called from the start method (which validates preconditions).
func (engine *engine) runWorker(name string, manifold Manifold, delay time.Duration) {

	// We snapshot the resources available at invocation time, rather than adding an
	// additional communicate-resource-request channel. The latter approach is not
	// unreasonable... but is prone to inelegant scrambles. For example:
	//
	//  * Install manifold A; loop starts worker A
	//  * Install manifold B; loop starts worker B
	//  * A communicates its worker back to loop; main thread bounces B
	//  * B asks for A, gets A, doesn't react to bounce (*)
	//  * B communicates its worker back to loop; loop kills it immediately in
	//    response to earlier bounce
	//  * loop starts worker B again, now everything's fine; but, still, yuck.
	//
	// The problem, of course, is in the (*); the main thread does know that B
	// needs to bounce, and it could communicate that fact back via an error
	// over a channel back into getResource; the StartFunc could then just return
	// (say) that ErrResourceChanged and avoid the hassle of creating a worker.
	//
	// But there's a fundamental race regardless -- we could *always* see a new
	// dependency land just after we cede control to user code in the dependent,
	// and at that point we have to bounce a fresh worker. Reducing occurrences
	// of this is laudable, but the complexity cost is too high for the benefits
	// we see; and the chosen appproach behaves well in the (common) scenario
	// detailed above:
	//
	//  * Install manifold A; loop starts worker A
	//  * Install manifold B; loop starts worker B with empty resource snapshot
	//  * A communicates its worker back to loop; main thread bounces B
	//  * B asks for A, gets nothing, can actually just return a degenerate
	//    worker that immediately exits nil (indicating "given the available
	//    dependencies I have done everything I can possibly do, and nothing
	//    actually went *wrong* specifically...").
	//  * loop restarts worker B with an up-to-date snapshot, B works fine
	//
	// We assume that, in the common case, most workers run without error most
	// of the time; and, thus, that the vast majority of worker startups will
	// happen as an agent starts. StartFuncs should be comfortable with
	// returning nil workers when hard dependencies are unmet; and workers
	// should be prepared to be stopped at any time, as they must already be.
	outputs := map[string]OutputFunc{}
	workers := map[string]worker.Worker{}
	for _, resourceName := range manifold.Inputs {
		outputs[resourceName] = engine.manifolds[resourceName].Output
		workers[resourceName] = engine.current[resourceName].worker
	}
	getResource := func(resourceName string, out interface{}) bool {
		switch {
		case workers[resourceName] == nil:
			return false
		case outputs[resourceName] == nil:
			return out == nil
		}
		return outputs[resourceName](workers[resourceName], out)
	}

	// run is defined separately from its invocation so that the handling of its
	// result -- which *must* be sent to engine.stopped -- stands out properly.
	run := func() error {
		logger.Infof("starting %s manifold worker in %s...", name, delay)
		select {
		case <-engine.tomb.Dying():
			logger.Infof("not starting %s manifold worker (shutting down)", name)
			return tomb.ErrDying
		case <-time.After(delay):
			logger.Infof("starting %s manifold worker", name)
		}

		worker, err := manifold.Start(getResource)
		if err != nil {
			logger.Infof("failed to start %s manifold worker: %v", name, err)
			return err
		}

		logger.Infof("running %s manifold worker: %v", name, worker)
		select {
		case <-engine.tomb.Dying():
			logger.Infof("stopping %s manifold worker (shutting down)", name)
			worker.Kill()
		case engine.started <- startedTicket{name, worker}:
			logger.Infof("registered %s manifold worker", name)
		}
		return worker.Wait()
	}

	// It is vital that this ticket be sent.
	engine.stopped <- stoppedTicket{name, run()}
}

// gotStarted updates the engine to reflect the creation of a worker. It must
// only be called from the loop goroutine.
func (engine *engine) gotStarted(name string, worker worker.Worker) {
	// Copy current info; check preconditions and abort the workers if we've
	// already been asked to stop it.
	info := engine.current[name]
	switch {
	case info.worker != nil:
		engine.tomb.Kill(errors.Errorf("fatal: unexpected %s manifold worker start", name))
		fallthrough
	case info.stopping, engine.isDying():
		logger.Infof("%s manifold worker no longer required", name)
		worker.Kill()
	default:
		// It's fine to use this worker; update info and copy back.
		logger.Infof("%s manifold worker started: %v", name, worker)
		info.starting = false
		info.worker = worker
		engine.current[name] = info

		// Any manifold that declares this one as an input needs to be restarted.
		engine.bounceDependents(name)
	}
}

// gotStopped updates the engine to reflect the demise of (or failure to create)
// a worker. It must only be called from the loop goroutine.
func (engine *engine) gotStopped(name string, err error) {
	logger.Infof("%s manifold worker stopped: %v", name, err)

	// Copy current info and check preconditions.
	info := engine.current[name]
	if info.stopped() {
		engine.tomb.Kill(errors.New("fatal: unexpected %s manifold worker stop"))
		return
	}

	// Reset engine info...
	engine.current[name] = workerInfo{}

	// ...and bail out if we can be sure there's no need to restart.
	if engine.isFatal(err) {
		engine.tomb.Kill(err)
		return
	}

	// If the worker stopped on its own, without error, it's finished its job
	// and won't be run again unless its dependencies change. Otherwise...
	if err != nil {
		// Something went wrong, but we don't much care what. Try again in a bit.
		engine.start(name, engine.errorDelay)
	} else if info.stopping {
		// We told it to stop, because its dependencies changed; we want to
		// start it again immediately.
		engine.start(name, engine.bounceDelay)
	}

	// Manifolds that declared a dependency on this one only need to be notified
	// if the worker has changed; if it was already nil, nobody needs to know.
	if info.worker != nil {
		engine.bounceDependents(name)
	}
}

// stop ensures that any running or starting worker will be stopped in the
// near future. It must only be called from the loop goroutine.
func (engine *engine) stop(name string) {

	// If already stopping or stopped, just don't do anything.
	info := engine.current[name]
	if info.stopping || info.stopped() {
		return
	}

	// Update info, kill worker if present, and copy info back to engine.
	info.stopping = true
	if info.worker != nil {
		info.worker.Kill()
	}
	engine.current[name] = info
}

// isDying returns true if the engine is shutting down.
func (engine *engine) isDying() bool {
	select {
	case <-engine.tomb.Dying():
		return true
	default:
		return false
	}
}

// allStopped returns true if no workers are running or starting.
func (engine *engine) allStopped() bool {
	for _, info := range engine.current {
		if !info.stopped() {
			return false
		}
	}
	return true
}

// bounceDependents starts every stopped dependent of the named manifold, and
// stops every started one (and trusts the rest of the engine to restart them).
// It must only be called from the loop goroutine.
func (engine *engine) bounceDependents(name string) {
	for _, name := range engine.dependents[name] {
		if engine.current[name].stopped() {
			engine.start(name, engine.bounceDelay)
		} else {
			engine.stop(name)
		}
	}
}
