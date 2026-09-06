package appointmentbooking

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
)

// Item implementa view.Itemizer — el ÚNICO código específico de view que carga este registro. El
// Presenter indexa las filas por ID a partir de esto durante Reload; no hay lookup manual byId/WithFill.
func (r *Reservation) Item() view.Item {
	return view.Item{
		ID:          r.Id,
		Label:       r.LocalStringDate + " " + r.LocalStringTime,
		Description: r.Status,
	}
}

// NewView construye el Presenter de Reservation — acotado al horario de un solo staff, ya que no
// existe una operación "listar todas las reservas de un tenant" sin acotar (ver
// docs/ARCHITECTURE.md §7). Solo lista: sin capacidad Saver/Deleter (las reservas solo mutan vía las
// transiciones FSM-guardadas de ChangeReservationStatus, y nunca se eliminan físicamente).
func NewView(caller router.Caller, tenantId, staffId string) view.Presenter {
	record := &Reservation{}

	return view.New(
		caller,
		record,
		OpListReservationsByStaff,
		func() model.ModelSlice { return &ReservationList{} },
		view.WithTitle("Reservas"),
		view.WithArgs(func() model.Encodable {
			return &ListReservationsByStaffArgs{TenantId: tenantId, StaffId: staffId}
		}),
	)
}
