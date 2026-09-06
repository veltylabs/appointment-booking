package appointmentbooking

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
)

// reservationLister adapts router.Caller + the staff-scoped list op to
// view.Lister. view.NewCallerLister is unusable here — it sends nil args,
// and this list is scoped to (tenantId, staffId).
type reservationLister struct {
	caller   router.Caller
	tenantId string
	staffId  string
}

func (l reservationLister) List() ([]model.Model, error) {
	out := &ReservationList{}
	ch := make(chan error, 1)
	l.caller.Call(
		OpListReservationsByStaff,
		&ListReservationsByStaffArgs{TenantId: l.tenantId, StaffId: l.staffId},
		out,
		func(err error) { ch <- err },
	)
	if err := <-ch; err != nil {
		return nil, err
	}
	rows := make([]model.Model, 0, out.Len())
	for i := 0; i < out.Len(); i++ {
		rows = append(rows, out.At(i).(*Reservation))
	}
	return rows, nil
}

var _ view.Lister = reservationLister{}
