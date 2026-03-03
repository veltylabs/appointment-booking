package appointmentbooking

type module struct{}

// icono svg del module contenido interno etiqueta svg
func (m module) IconSvg() map[string]string {
	return map[string]string{
		"appointment-booking-module": `<path fill="currentColor" d="m10.29 11.71-3.293-3.293v-4.414h2v3.586l2.707 2.707zm-2.293-11.71c-4.418 0-8 3.582-8 8s3.582 8 8 8 8-3.582 8-8-3.582-8-8-8zm0 14c-3.314 0-6-2.686-6-6s2.686-6 6-6 6 2.686 6 6-2.686 6-6 6z"/>`,
	}
}
