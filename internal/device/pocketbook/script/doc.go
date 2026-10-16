// Package script drives the app with scripts of taps, buttons and waits, and saves shots of the
// panel. The emulator runs them headless, against an emulated reader; the app's remote control
// runs them on a real reader, sent over Wi-Fi. Both are Targets: something that takes touches
// and button presses as the firmware would deliver them, and shows what the panel shows.
//
// A script has one command per line ("#" starts a comment), coordinates in screen pixels. It
// starts once the app is idle (see idle). A touch or a button waits until the app has handled
// the input before and finished the work it started (app.App's BackgroundWork: a download,
// loading a list, preloading pages), then only as long as a finger or a button would. After a
// step that changes the screen, use idle or shot, which also wait for the panel to stop
// changing.
//
//	tap X Y                 touch and lift
//	long X Y                touch, hold past the long press delay, lift
//	swipe X1 Y1 X2 Y2       touch, move, lift
//	key NAME                press and release a button (see KeyNames)
//	hold NAME DURATION      press a button, release it after DURATION
//	home, resume            send the app to the background, bring it back
//	battery PERCENT [charging]
//	net up|down             turn the emulated network on or off, with the firmware's events
//	dark on|off             switch the emulated dark mode setting, which the app reads when it
//	                        comes back to the foreground: follow it with home and resume
//	wait DURATION           sleep
//	idle [TIMEOUT]          wait until the app is idle: its input handled, its work over and
//	                        the panel unchanged for quietTime (default timeout 2m)
//	shot NAME               idle, then save the panel as NAME
//	exit                    ask the app to exit, as the firmware does; the script ends there
//
// home, resume, battery, net and dark change the emulated reader itself: they run only on a
// Target that is also Emulated, and fail on a real reader.
package script
