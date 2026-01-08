//go:build linux && cgo

package crun

/*
#include "go_crun.h"
*/
import "C"
import (
	"encoding/json"
	"runtime"
	"unsafe"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// ContainerSpec wraps libcrun_container_t holding the OCI spec.
// This is the spec holder - create a Container via RuntimeContext.Create/Run.
type ContainerSpec struct {
	c *C.libcrun_container_t
}

// LoadContainerSpecFromFile loads an OCI spec from file.
func LoadContainerSpecFromFile(path string) (*ContainerSpec, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var err C.libcrun_error_t
	ctr := C.libcrun_container_load_from_file(cpath, &err)
	if ctr == nil {
		return nil, fromLibcrunErr(&err)
	}
	c := &ContainerSpec{c: ctr}
	runtime.SetFinalizer(c, func(cc *ContainerSpec) { _ = cc.Close() })
	return c, nil
}

// LoadContainerSpecFromJSON loads an OCI spec from a JSON string.
func LoadContainerSpecFromJSON(def string) (*ContainerSpec, error) {
	cdef := C.CString(def)
	defer C.free(unsafe.Pointer(cdef))
	var err C.libcrun_error_t
	ctr := C.libcrun_container_load_from_memory(cdef, &err)
	if ctr == nil {
		return nil, fromLibcrunErr(&err)
	}
	c := &ContainerSpec{c: ctr}
	runtime.SetFinalizer(c, func(cc *ContainerSpec) { _ = cc.Close() })
	return c, nil
}

// NewContainerSpec creates a ContainerSpec from a typed specs.Spec.
func NewContainerSpec(sp *specs.Spec) (*ContainerSpec, error) {
	b, err := json.Marshal(sp)
	if err != nil {
		return nil, err
	}
	return LoadContainerSpecFromJSON(string(b))
}

// Close releases the heavy spec memory associated with the ContainerSpec.
func (c *ContainerSpec) Close() error {
	if c == nil || c.c == nil {
		return nil
	}
	C.go_crun_free_container(c.c)
	c.c = nil
	return nil
}

// Spec returns a baseline OCI config JSON. Set rootless to true for a rootless template.
func Spec(rootless bool) (string, error) {
	var err C.libcrun_error_t
	var ln C.int
	buf := C.go_crun_spec_json(C.bool(rootless), &ln, &err)
	if buf == nil {
		return "", fromLibcrunErr(&err)
	}
	defer C.free(unsafe.Pointer(buf))
	return C.GoStringN(buf, ln), nil
}

