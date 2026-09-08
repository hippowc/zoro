//go:build darwin && cgo

package hotkey

/*
#cgo LDFLAGS: -framework Carbon

#include <Carbon/Carbon.h>
#include <dispatch/dispatch.h>

extern void goZoroHotKey(void);

static EventHotKeyRef gZoroHotKeyRef = NULL;
static EventHandlerRef gZoroHandlerRef = NULL;

// Carbon event handler: fires on the main run loop whenever the registered
// hotkey (Cmd+Shift+Z) is pressed.
static OSStatus zoroHotKeyHandler(EventHandlerCallRef callRef, EventRef event, void* userData) {
	(void)callRef;
	(void)userData;

	EventHotKeyID hotKeyID;
	OSStatus err = GetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID,
		NULL, sizeof(hotKeyID), NULL, &hotKeyID);
	if (err == noErr && hotKeyID.signature == 'zoro' && hotKeyID.id == 1) {
		goZoroHotKey();
	}
	return noErr;
}

static void zoroInstallHotKey(void) {
	EventTypeSpec eventType = { kEventClassKeyboard, kEventHotKeyPressed };
	EventHandlerUPP handlerUPP = NewEventHandlerUPP(zoroHotKeyHandler);
	InstallEventHandler(GetApplicationEventTarget(), handlerUPP, 1, &eventType, NULL, &gZoroHandlerRef);

	EventHotKeyID hotKeyID = { 'zoro', 1 };
	// kVK_ANSI_Z == 6; cmdKey | shiftKey == 0x0100 | 0x0200.
	RegisterEventHotKey((UInt32)kVK_ANSI_Z, (UInt32)(cmdKey | shiftKey),
		hotKeyID, GetApplicationEventTarget(), 0, &gZoroHotKeyRef);
}

static void zoroUninstallHotKey(void) {
	if (gZoroHotKeyRef != NULL) {
		UnregisterEventHotKey(gZoroHotKeyRef);
		gZoroHotKeyRef = NULL;
	}
	if (gZoroHandlerRef != NULL) {
		RemoveEventHandler(gZoroHandlerRef);
		gZoroHandlerRef = NULL;
	}
}

static void zoroInstallThunk(void* ctx) {
	(void)ctx;
	zoroInstallHotKey();
}

static void zoroUninstallThunk(void* ctx) {
	(void)ctx;
	zoroUninstallHotKey();
}

// Carbon event handlers must be installed on the main thread; Wails owns the
// main run loop, so we hop onto the main queue via GCD (no blocks required).
static void zoroInstallHotKeyAsync(void) {
	dispatch_async_f(dispatch_get_main_queue(), NULL, zoroInstallThunk);
}

static void zoroUninstallHotKeyAsync(void) {
	dispatch_async_f(dispatch_get_main_queue(), NULL, zoroUninstallThunk);
}
*/
import "C"

// trigger is fed by exported Go callback `goZoroHotKey` (invoked from C).
var trigger = make(chan struct{}, 8)

//export goZoroHotKey
func goZoroHotKey() {
	select {
	case trigger <- struct{}{}:
	default:
	}
}

// Listen registers the default summon hotkey Cmd+Shift+Z and returns a channel
// that receives a value on every press plus a best-effort unregister func.
func Listen() (<-chan struct{}, func(), error) {
	C.zoroInstallHotKeyAsync()
	once := false
	unregister := func() {
		if once {
			return
		}
		once = true
		C.zoroUninstallHotKeyAsync()
	}
	return trigger, unregister, nil
}
