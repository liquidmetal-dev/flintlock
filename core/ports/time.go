package ports

import "time"

type HasTime interface {
	SetClock(func() time.Time)
}
