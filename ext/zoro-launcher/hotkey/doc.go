// Package hotkey provides the global hotkey used to summon the zoro launcher.
//
// On macOS it uses Carbon RegisterEventHotKey, which needs no
// Accessibility/Input-Monitoring permission. Other platforms are stubbed for
// now (the launcher MVP targets macOS first).
package hotkey
