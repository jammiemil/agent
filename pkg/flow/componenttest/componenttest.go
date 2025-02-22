// Package componenttest provides utilities for testing Flow components.
package componenttest

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/go-kit/log"
	"github.com/grafana/agent/component"
)

// A Controller is a testing controller which controls a single component.
type Controller struct {
	reg component.Registration
	log log.Logger

	onRun   sync.Once
	running chan struct{}

	innerMut sync.Mutex
	inner    component.Component

	exportsMut sync.Mutex
	exports    component.Exports
	exportsCh  chan struct{}
}

// NewControllerFromID returns a new testing Controller for the component with
// the provided name.
func NewControllerFromID(l log.Logger, componentName string) (*Controller, error) {
	if l == nil {
		l = log.NewNopLogger()
	}

	reg, ok := component.Get(componentName)
	if !ok {
		return nil, fmt.Errorf("no such component %q", componentName)
	}

	return &Controller{
		reg: reg,
		log: l,

		running:   make(chan struct{}, 1),
		exportsCh: make(chan struct{}, 1),
	}, nil
}

func (c *Controller) onStateChange(e component.Exports) {
	c.exportsMut.Lock()
	changed := !reflect.DeepEqual(c.exports, e)
	c.exports = e
	c.exportsMut.Unlock()

	if !changed {
		return
	}

	select {
	case c.exportsCh <- struct{}{}:
	default:
	}
}

// WaitRunning blocks until the Controller is running up to the provided
// timeout.
func (c *Controller) WaitRunning(timeout time.Duration) error {
	select {
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for the controller to start running")
	case <-c.running:
		return nil
	}
}

// WaitExports blocks until new Exports are available up to the provided
// timeout.
func (c *Controller) WaitExports(timeout time.Duration) error {
	select {
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for exports")
	case <-c.exportsCh:
		return nil
	}
}

// Exports gets the most recent exports for a component.
func (c *Controller) Exports() component.Exports {
	c.exportsMut.Lock()
	defer c.exportsMut.Unlock()
	return c.exports
}

// Run starts the controller, building and running the component. Run blocks
// until ctx is canceled, the component exits, or if there was an error.
//
// Run may only be called once per Controller.
func (c *Controller) Run(ctx context.Context, args component.Arguments) error {
	dataPath, err := os.MkdirTemp("", "controller-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(dataPath)
	}()

	run, err := c.buildComponent(dataPath, args)
	if err != nil {
		return err
	}

	c.onRun.Do(func() { close(c.running) })
	return run.Run(ctx)
}

func (c *Controller) buildComponent(dataPath string, args component.Arguments) (component.Component, error) {
	c.innerMut.Lock()
	defer c.innerMut.Unlock()

	opts := component.Options{
		ID:            c.reg.Name + ".test",
		Logger:        c.log,
		DataPath:      dataPath,
		OnStateChange: c.onStateChange,
		Registerer:    prometheus.NewRegistry(),
	}

	inner, err := c.reg.Build(opts, args)
	if err != nil {
		return nil, err
	}

	c.inner = inner
	return inner, nil
}

// Update updates the running component. Should only be called after Run.
func (c *Controller) Update(args component.Arguments) error {
	c.innerMut.Lock()
	defer c.innerMut.Unlock()

	if c.inner == nil {
		return fmt.Errorf("component is not running")
	}
	return c.inner.Update(args)
}
