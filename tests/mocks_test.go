package tests

import (
	"github.com/tinywasm/events"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	ab "github.com/veltylabs/appointment_booking"
)

type MockStaffReader struct {
	Exists bool
	Err    error
}

func (m *MockStaffReader) StaffExists(tenantID, staffID string) (bool, error) {
	return m.Exists, m.Err
}

type MockCatalogReader struct {
	Exists bool
	Err    error
}

func (m *MockCatalogReader) ServiceExists(tenantID, serviceID string) (bool, error) {
	return m.Exists, m.Err
}

type MockDirectoryReader struct {
	Exists bool
	Err    error
}

func (m *MockDirectoryReader) ClientExists(tenantID, clientID string) (bool, error) {
	return m.Exists, m.Err
}

type MockEventPublisher struct {
	PublishedEvents []string
}

func (m *MockEventPublisher) Publish(e events.Event) {
	m.PublishedEvents = append(m.PublishedEvents, e.Topic)
}

var _ events.Publisher = (*MockEventPublisher)(nil)

type fakeIDs struct{ n int }

func (f *fakeIDs) NewID() string {
	f.n++
	return "test-id-" + fmt.Convert(f.n).String()
}

var _ model.IDGenerator = (*fakeIDs)(nil)

func SetupDependencies() ab.Deps {
	return ab.Deps{
		Staff:     &MockStaffReader{Exists: true},
		Catalog:   &MockCatalogReader{Exists: true},
		Directory: &MockDirectoryReader{Exists: true},
		IDs:       &fakeIDs{},
		Publisher: &MockEventPublisher{},
	}
}
