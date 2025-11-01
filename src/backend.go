package main

type MountOptions struct {
	ReadWrite bool
	CDROM     bool
}

type MountStatus struct {
	Mounted  bool
	File     string
	ReadOnly bool
	CDROM    bool
}

type Backend interface {
	Name() string
	Supported() bool
	Mount(isoPath string, opts MountOptions) error
	Unmount() error
	Status() (*MountStatus, error)
}
