package buffer

import "errors"

var (
	ErrorElementUnallocated = errors.New("encountered nil in the buffer. this should not happen. check usage")
	ErrorChannelBufferClose = errors.New("channel buffer has be closed. cannot perform this operation")
)
