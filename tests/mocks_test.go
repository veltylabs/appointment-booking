package tests

import (
	"context"
	ab "github.com/veltylabs/appointment-booking"
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
	Err             error
}

func (m *MockEventPublisher) Publish(ctx context.Context, event string, payload any) error {
	m.PublishedEvents = append(m.PublishedEvents, event)
	return m.Err
}

func SetupDependencies() ab.Deps {
	return ab.Deps{
		Staff:     &MockStaffReader{Exists: true},
		Catalog:   &MockCatalogReader{Exists: true},
		Directory: &MockDirectoryReader{Exists: true},
		Publisher: &MockEventPublisher{},
	}
}
