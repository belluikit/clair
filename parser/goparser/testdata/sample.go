// Package sample is a test fixture for the Go parser.
package sample

import (
	"context"

	"example.com/framework/widget"
)

// Widget is a sample widget that wraps a framework list.
type Widget struct {
	// Name is the widget display name.
	Name string

	// List is the underlying framework list.
	List *widget.List

	// internal is unexported and should be ignored.
	internal int
}

// Renderer is the interface that widgets must implement.
type Renderer interface {
	// Render draws the widget.
	Render() error

	// MinSize returns the minimum size.
	MinSize() Size
}

// Size represents dimensions.
type Size struct {
	Width  float64
	Height float64
}

// MaxItems is the maximum number of items.
const MaxItems = 100

// DefaultName is the default widget name.
var DefaultName = "untitled"

// unexportedConst should be ignored.
const unexportedConst = 42

// NewWidget creates a new Widget.
func NewWidget(name string, list *widget.List) *Widget {
	return &Widget{Name: name, List: list}
}

// Render draws the widget.
func (w *Widget) Render() error {
	return nil
}

// SetData updates the widget's data binding.
func (w *Widget) SetData(ctx context.Context) error {
	return nil
}

// unexportedFunc should be ignored.
func unexportedFunc() {}
