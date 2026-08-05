package appointmentbooking

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
)

// NewView builds the Reservation Presenter — scoped to one staff member's schedule, since there is
// no unscoped "list all reservations for a tenant" op (see docs/ARCHITECTURE.md §7). List-only: no
// Saver/Deleter capability (reservations mutate only via ChangeReservationStatus's FSM-gated
// transitions, and are never hard-deleted).
func NewView(caller router.Caller, tenantId, staffId string) view.Presenter {
	// Cache privado como SLICE con scan lineal — NO map: la regla "cero map" de AGENTS.md no
	// tiene excepciones (ni siquiera estado privado; el runtime de map de TinyGo viaja en el
	// binario wasm igual). Mismo patrón que item_catalog/view.go tras su review.
	var byId []*Reservation
	record := &Reservation{}

	return view.New(
		caller,
		record,
		OpListReservationsByStaff,
		func() model.FielderSlice { return &ReservationList{} },
		func(list model.FielderSlice) []view.Item {
			l := list.(*ReservationList)
			items := make([]view.Item, l.Len())
			byId = make([]*Reservation, l.Len())
			for i := 0; i < l.Len(); i++ {
				it := l.At(i).(*Reservation)
				byId[i] = it
				items[i] = view.Item{
					ID:          it.Id,
					Label:       it.LocalStringDate + " " + it.LocalStringTime,
					Description: it.Status,
				}
			}
			return items
		},
		view.WithTitle("Reservas"),
		view.WithArgs(func() model.Encodable {
			return &ListReservationsByStaffArgs{TenantId: tenantId, StaffId: staffId}
		}),
		view.WithFill(func(id string) model.Model {
			if id == "" {
				return nil
			}
			for _, it := range byId {
				if it != nil && it.Id == id {
					return it
				}
			}
			return nil
		}),
	)
}
