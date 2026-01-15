package buffer

import "errors"

var (
	ErrorChannelBufferClose = errors.New("channel buffer has be closed. cannot perform this operation")
)
