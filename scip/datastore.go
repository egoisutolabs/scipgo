package scip

/*
#include "helpers.h"
*/
import "C"

import "reflect"

// datastore implements russcip's "datastore" feature: arbitrary typed user
// data attached to a model. Instead of hiding the data in an unused plugin
// like the Rust version does, a Go-side registry keyed by the raw SCIP
// pointer is used, cleaned up when the SCIP instance is freed. Keying by the
// raw pointer makes data visible both from the owning Model and from the
// Model wrappers handed to plugin callbacks.
type datastore struct {
	mu    chan struct{}
	items map[any]any
}

func newDatastore() *datastore {
	return &datastore{mu: make(chan struct{}, 1), items: make(map[any]any)}
}

func (d *datastore) lock()   { d.mu <- struct{}{} }
func (d *datastore) unlock() { <-d.mu }

var datastores = make(map[*C.SCIP]*datastore)

var datastoresMu = make(chan struct{}, 1)

func datastoresLock()   { datastoresMu <- struct{}{} }
func datastoresUnlock() { <-datastoresMu }

func getDatastore(raw *C.SCIP) *datastore {
	raw = rootScip(raw) // sub-SCIP copies share their origin's data
	datastoresLock()
	defer datastoresUnlock()
	ds, ok := datastores[raw]
	if !ok {
		ds = newDatastore()
		datastores[raw] = ds
	}
	return ds
}

func deleteDatastore(s *Scip) {
	if s.raw == nil {
		return
	}
	datastoresLock()
	defer datastoresUnlock()
	delete(datastores, s.raw)
}

// SetData attaches arbitrary typed data to the model. Existing data of the
// same type is replaced. Store a pointer to mutate the attached data later.
func SetData[D any](m Model, data D) {
	ds := getDatastore(m.scip.raw)
	ds.lock()
	defer ds.unlock()
	ds.items[typeKey[D]()] = data
}

// typeKey keys the store by the static type D (like anymap in russcip), so
// interface types work: reflect.TypeOf of a nil interface value is nil.
func typeKey[D any]() reflect.Type { return reflect.TypeOf((*D)(nil)).Elem() }

// GetData retrieves data of type D attached to the model. The second return
// value reports whether data of this type was attached.
func GetData[D any](m Model) (D, bool) {
	var zero D
	ds := getDatastore(m.scip.raw)
	ds.lock()
	defer ds.unlock()
	v, ok := ds.items[typeKey[D]()]
	if !ok {
		return zero, false
	}
	d, ok := v.(D)
	return d, ok
}

// MustGetData retrieves data of type D attached to the model, panicking when
// absent. Useful inside plugin callbacks.
func MustGetData[D any](m Model) D {
	d, ok := GetData[D](m)
	if !ok {
		panic("scip: no data of this type attached to model")
	}
	return d
}
