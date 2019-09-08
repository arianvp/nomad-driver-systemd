package systemd

import "sync"
import "github.com/hashicorp/nomad/plugins/drivers"
import "github.com/coreos/go-systemd/dbus"

type taskHandle struct {
	subscription *dbus.SubscriptionSet
	handle       *drivers.TaskHandle
}

// a typesafe adapter of sync.Map
// it's a sync.Map<*taskHandle>
type taskStore struct {
	store sync.Map
}

func newTaskStore() *taskStore {
	return &taskStore{store: sync.Map{}}
}

func (ts *taskStore) Set(id string, handle *taskHandle) {
	ts.store.Store(id, handle)
}

func (ts *taskStore) Get(id string) (*taskHandle, bool) {
	v, ok := ts.store.Load(id)
	if ok {
		return v.(*taskHandle), ok
	} else {
		return nil, ok
	}
}

func (ts *taskStore) Delete(id string) {
	ts.store.Delete(id)
}
