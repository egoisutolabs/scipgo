package scip

import "testing"

type myData struct {
	title string
}

func (d *myData) Title() string { return d.title }

type titled interface{ Title() string }

func TestDatastore(t *testing.T) {
	model := NewModel()

	// immutable data
	SetData(model, 5)
	if n, ok := GetData[int](model); !ok || n != 5 {
		t.Fatalf("got %v ok=%v", n, ok)
	}
	if _, ok := GetData[string](model); ok {
		t.Fatal("unexpected string data")
	}

	// mutable data: store a pointer
	SetData(model, &myData{title: "My Data"})
	d, ok := GetData[*myData](model)
	if !ok || d.title != "My Data" {
		t.Fatalf("got %+v ok=%v", d, ok)
	}
	d.title = "New Title"
	if d, _ = GetData[*myData](model); d.title != "New Title" {
		t.Fatalf("got %+v", d)
	}
}

func TestDatastoreInterfaceType(t *testing.T) {
	model := NewModel()
	SetData[titled](model, &myData{title: "iface"})
	d, ok := GetData[titled](model)
	if !ok || d.Title() != "iface" {
		t.Fatalf("got %v ok=%v", d, ok)
	}
}
