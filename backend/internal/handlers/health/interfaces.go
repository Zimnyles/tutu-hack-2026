package health_handler

import "context"

type Checker interface {
	Check(context.Context) error
}
