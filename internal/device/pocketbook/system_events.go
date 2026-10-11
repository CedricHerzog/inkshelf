package pocketbook

// SystemEvent is news from the firmware that the app may act on, passed to App.System.
type SystemEvent int

// The system events.
const (
	SaveState       SystemEvent = iota // save what must survive being killed
	StorageChanged                     // files changed on a storage (a card, or after USB)
	USBStorageIn                       // the reader is exported over USB: its storage may go away
	USBStorageOut                      // the reader is back from USB
	NetConnected                       // the network came up
	NetDisconnected                    // the network went down
)

// systemEvents maps the firmware's numbers of the task events passed on to App.System to the
// app's SystemEvents. The event loop handles the other task events itself, or ignores them.
var systemEvents = map[int32]SystemEvent{
	eventSaveState:       SaveState,
	eventFSChanged:       StorageChanged,
	eventUSBStoreIn:      USBStorageIn,
	eventUSBStoreOut:     USBStorageOut,
	eventNetConnected:    NetConnected,
	eventNetDisconnected: NetDisconnected,
}

// String returns the event's name, for the log.
func (event SystemEvent) String() string {
	switch event {
	case SaveState:
		return "save state"
	case StorageChanged:
		return "storage changed"
	case USBStorageIn:
		return "USB storage in"
	case USBStorageOut:
		return "USB storage out"
	case NetConnected:
		return "net connected"
	case NetDisconnected:
		return "net disconnected"
	default:
		return "unknown"
	}
}
