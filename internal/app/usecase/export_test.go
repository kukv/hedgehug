package usecase

// SharesTokensWith reports whether ss and o use one token cache.
func (ss *Session) SharesTokensWith(o *Session) bool {
	return ss.tokens != nil && ss.tokens == o.tokens
}

// WriterCached reports whether ss has read the writer and keeps it.
func (ss *Session) WriterCached() bool { return ss.writerDone }
