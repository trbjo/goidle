# Goidle – A Wayland Idle Daemon

Goidle is an idle management solution for Wayland compositors that implements output management, brightness control, and the `ext_idle_notify_v1` protocol.

## Features

- Output on/off management
- Brightness management
- Implementation of `ext_idle_notify_v1` protocol
- Configurable idle timeouts for locked and unlocked states
- Automatic dimming and restoring of brightness as an idle indicator
- DBus API for system control

## Configuration

Goidle automatically generates a default configuration file on its first run. The file is located at `~/.config/goidle.json` by default. You can specify a different path using the `GOIDLE_CONFIG` environment variable.

The generated configuration will look similar to this:
```json
{
    "backlight_curve_factor": 0.5,
    "backlight_dim_ratio": 0.2,
    "backlight_steps": 16,
    "idle_grace_duration": "30s",
    "lock_command": ["hyprlock"],
    "trusted_wifi_networks": [],
    "timeout_active_dim": "150s",
    "timeout_active_to_idle": "180s",
    "timeout_idle_backlight_off": "15s",
    "timeout_idle_to_suspend": "20s",
    "lock_init_ignore_input_timeout": "1s"
}
```

You can modify these values in the generated file to customize Goidle's behavior. The configuration file is in JSON format and will be read by Goidle on subsequent runs.


## DBus API

Goidle exposes the following DBus calls:

| Call | Description |
|------|-------------|
| `Suspend` | Puts the system into suspend mode |
| `Lock` | Locks the screen |
| `LidClose` | Simulates a laptop lid close event |
| `LidOpen` | Simulates a laptop lid open event |
| `WifiTrust` | Adds current WiFi to trusted networks |
| `WifiDistrust` | Removes current WiFi from trusted networks |
| `LogDebug` | Sets log level to Debug |
| `LogWarn` | Sets log level to Warning |
| `LogInfo` | Sets log level to Info |
| `IdleGraceDuration` | Sets the grace period before entering idle state |
| `ToggleOutput` | Toggles the display output on/off |
| `IdleInhibit` | Prevents the system from entering idle state |
| `IdleAllow` | Allows the system to enter idle state |
| `LightIncrease` | Increases screen brightness |
| `LightDecrease` | Decreases screen brightness |

## Compilation

To compile Goidle, use the following command:
```bash
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags '-s -w -extldflags "-static"' .
```

## Usage

After compilation, run the `goidle` binary. Ensure your Wayland compositor supports the `ext_idle_notify_v1` protocol.

## Keybindings

Here's an example of how to add keybindings in Sway to interact with Goidle:

```bash
# goidle
bindsym --locked XF86MonBrightnessDown exec dbus-send --print-reply --dest=io.github.trbjo.GoIdle /io/github/trbjo/GoIdle io.github.trbjo.GoIdle.LightDecrease
bindsym --locked XF86MonBrightnessUp exec dbus-send --print-reply --dest=io.github.trbjo.GoIdle /io/github/trbjo/GoIdle io.github.trbjo.GoIdle.LightIncrease
bindsym --locked F1 exec dbus-send --print-reply --dest=io.github.trbjo.GoIdle /io/github/trbjo/GoIdle io.github.trbjo.GoIdle.LightDecrease
bindsym --locked F2 exec dbus-send --print-reply --dest=io.github.trbjo.GoIdle /io/github/trbjo/GoIdle io.github.trbjo.GoIdle.LightIncrease

bindsym --locked XF86PowerOff exec dbus-send --type=method_call --print-reply --dest=io.github.trbjo.GoIdle /io/github/trbjo/GoIdle io.github.trbjo.GoIdle.Suspend
bindswitch --locked lid:on exec exec dbus-send --type=method_call --print-reply --dest=io.github.trbjo.GoIdle /io/github/trbjo/GoIdle io.github.trbjo.GoIdle.LidClose
bindswitch --locked lid:off exec exec dbus-send --type=method_call --print-reply --dest=io.github.trbjo.GoIdle /io/github/trbjo/GoIdle io.github.trbjo.GoIdle.LidOpen
bindsym --no-repeat --locked $super+l exec dbus-send --session --type=method_call --print-reply --dest=io.github.trbjo.GoIdle /io/github/trbjo/GoIdle io.github.trbjo.GoIdle.ToggleOutput string:"eDP-1"
```

## Contributing

Contributions are welcome. Please feel free to submit a Pull Request.

## License

MIT License

Copyright (c) 2024 Troels Bjørnskov

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

## Support

For issues, feature requests, or questions, please open an issue on the GitHub repository.
