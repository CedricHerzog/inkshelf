# Contract: screen orientation (device/pocketbook)

What `device/pocketbook` offers the app so a screen can be drawn in landscape. Names are
indicative; the device stays free of EPUB and spread concepts (principle II).

## Operations

- `SetOrientation(Portrait | Landscape)`: called on the main thread only, inside a draw.
  Afterwards `ScreenSize()` and `Screen()` return the canvas of that orientation (1404x1872 or
  1872x1404), touches arrive in its coordinates, and the next refresh is a full flash refresh.
  Setting the current orientation does nothing.
- `Orientation()`: the current one.
- Going to the background (firmware suppress or task switch) returns to portrait first, so the
  firmware's task thumbnail and the next app see a portrait screen.

## Guarantees

- Portrait behaves exactly as today: `copyRotated` and the touch mapping are untouched, and
  existing shots are pixel-identical.
- Landscape copies the canvas to the framebuffer without rotation (`copyStraight`, with a
  benchmark), in the direction found on the reader; touches map the same way.
- The emulator honors the same calls: its panel, its shots and its web view take the landscape
  size. Script coordinates are checked against the current size.

## Used by

`app/screens/reader` only: landscape for a spread, portrait for anything else. Every other
screen draws in portrait; the app sets portrait before drawing one.
