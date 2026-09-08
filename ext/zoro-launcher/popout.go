package main

// PopoutManager is a stub for future Wails v3 multi-window support.
// Currently unused as we use Preview.app/browser for popouts.
type PopoutManager struct{}

// NewPopoutManager creates a new popout manager (stub).
func NewPopoutManager() *PopoutManager {
	return &PopoutManager{}
}

// Create is a stub - currently popouts use system browser/Preview.app.
func (pm *PopoutManager) Create(html string) error {
	// This will be implemented when upgrading to Wails v3 with native multi-window support
	return nil
}

// CloseAll is a stub.
func (pm *PopoutManager) CloseAll() {
	// Nothing to close in this implementation
}

// Count returns 0 (stub).
func (pm *PopoutManager) Count() int {
	return 0
}
