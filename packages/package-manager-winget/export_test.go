package main

// UseFakeExitCodes maps recorder fake exit statuses (0-255) to the HRESULTs
// winget returns, until the returned function restores the real mapping.
func UseFakeExitCodes(codes map[int]uint32) (restore func()) {
	fakeExitCodes = codes
	return func() { fakeExitCodes = nil }
}

// HResult exposes the exit-status conversion for specs.
func HResult(err error) (uint32, bool) { return hresult(err) }
