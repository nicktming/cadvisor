package pages

const ContainersPage = "/containers/"


type ByteSize float64

const (
	_ 	= iota

	KB 	ByteSize = 1 << (10 * iota)
)