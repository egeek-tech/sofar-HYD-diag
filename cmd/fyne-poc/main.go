// Fyne v2 native desktop PoC for Sofar HYD inverter monitoring.
// Demonstrates per-parameter streaming: each register value updates individually
// as its Modbus read returns, providing visible ~500ms update cadence per parameter.
package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"image/color"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"sofar-hyd-diag/internal/broker"
	"sofar-hyd-diag/internal/register"
)

var (
	inverterHost = flag.String("inverter-host", "10.5.99.29", "Inverter IP address")
	inverterPort = flag.Int("inverter-port", 4192, "Inverter TCP port")
	slaveID      = flag.Int("slave", 1, "Modbus slave ID (1-247)")
	modbusMode   = flag.String("modbus-mode", "tcp", "Modbus protocol mode: tcp or rtu")
	logLevel     = flag.String("log-level", "info", "Log level: debug, info, warn, error")
)

// System time register addresses for composite datetime display.
const (
	addrTimeYear  = 0x042C
	addrTimeMonth = 0x042D
	addrTimeDay   = 0x042E
	addrTimeHour  = 0x042F
	addrTimeMin   = 0x0430
	addrTimeSec   = 0x0431
)

// stateColor returns a color for each broker connection state.
func stateColor(s broker.State) color.Color {
	switch s {
	case broker.StateConnected:
		return color.NRGBA{R: 34, G: 139, B: 34, A: 255} // green
	case broker.StateConnecting, broker.StateReconnecting:
		return color.NRGBA{R: 218, G: 165, B: 32, A: 255} // goldenrod
	case broker.StateDisconnected:
		return color.NRGBA{R: 178, G: 34, B: 34, A: 255} // red
	default: // dormant
		return color.NRGBA{R: 128, G: 128, B: 128, A: 255} // gray
	}
}

// isTimeRegister returns true if the probe address is one of the 6 system time registers.
func isTimeRegister(addr uint16) bool {
	return addr >= addrTimeYear && addr <= addrTimeSec
}

// setupLogger creates an slog.Logger with the specified log level.
func setupLogger(levelName string) *slog.Logger {
	var level slog.LevelVar
	switch strings.ToLower(levelName) {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: &level})
	return slog.New(handler)
}

func main() {
	flag.Parse()

	// Validate inputs
	if *slaveID < 1 || *slaveID > 247 {
		fmt.Fprintf(os.Stderr, "error: slave ID must be 1-247, got %d\n", *slaveID)
		os.Exit(1)
	}
	if *modbusMode != "tcp" && *modbusMode != "rtu" {
		fmt.Fprintf(os.Stderr, "error: modbus-mode must be 'tcp' or 'rtu', got %q\n", *modbusMode)
		os.Exit(1)
	}

	logger := setupLogger(*logLevel)

	// Create Fyne application and window
	fyneApp := app.New()
	win := fyneApp.NewWindow("Sofar HYD - System Monitor (PoC)")
	win.Resize(fyne.NewSize(700, 600))

	// Create broker in dormant state
	inverterAddr := fmt.Sprintf("%s:%d", *inverterHost, *inverterPort)
	useRTU := *modbusMode == "rtu"
	b := broker.New(logger.With("component", "broker"), inverterAddr, byte(*slaveID), useRTU)

	// Context for broker and polling goroutines
	ctx, cancel := context.WithCancel(context.Background())

	// Start broker command loop
	go b.Run(ctx)

	// === Connection bar ===

	statusIndicator := canvas.NewCircle(stateColor(broker.StateDormant))
	statusIndicator.Resize(fyne.NewSize(12, 12))

	statusLabel := widget.NewLabel("dormant")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	connectBtn := widget.NewButton("Connect", nil)

	// Polling cancellation for the streaming goroutine
	var pollCancel context.CancelFunc
	var pollMu sync.Mutex

	// Value labels keyed by probe address for targeted updates
	valueLabels := make(map[uint16]*widget.Label)

	// Special composite label for system time
	timeLabel := widget.NewLabel("---")
	timeLabel.Alignment = fyne.TextAlignTrailing

	// Time accumulator for composing 6 registers into one datetime string
	var timeMu sync.Mutex
	timeVals := make(map[uint16]uint16) // addr -> raw uint16 value

	// === Build system parameter sections ===

	sections := container.NewVBox()

	for _, group := range register.SystemGroups {
		// Group header
		header := widget.NewLabel(group.Name)
		header.TextStyle = fyne.TextStyle{Bold: true}
		sections.Add(header)

		// Track whether we already added the composite time row for this group
		timeRowAdded := false

		grid := container.New(layout.NewFormLayout())

		for _, probe := range group.Probes {
			if isTimeRegister(probe.Addr) {
				// Add a single "System time" row for the first time register encountered
				if !timeRowAdded {
					nameLabel := widget.NewLabel("System time")
					grid.Add(nameLabel)
					grid.Add(timeLabel)
					timeRowAdded = true
				}
				// Still create per-address entries in the map so we can read them individually
				// but do NOT create separate display rows for each time register.
				valueLabels[probe.Addr] = timeLabel // point all time probes to the shared label
				continue
			}

			nameLabel := widget.NewLabel(probe.Name)

			valLabel := widget.NewLabel("---")
			valLabel.Alignment = fyne.TextAlignTrailing
			valueLabels[probe.Addr] = valLabel

			grid.Add(nameLabel)
			grid.Add(valLabel)
		}

		sections.Add(grid)

		// Add a separator between groups
		sections.Add(widget.NewSeparator())
	}

	scrollContent := container.NewVScroll(sections)

	// resetAllValues sets all value labels back to "---".
	resetAllValues := func() {
		for addr, lbl := range valueLabels {
			if isTimeRegister(addr) {
				continue // handled separately
			}
			lbl.SetText("---")
		}
		timeLabel.SetText("---")
		timeMu.Lock()
		for k := range timeVals {
			delete(timeVals, k)
		}
		timeMu.Unlock()
	}

	// startPolling launches the per-parameter streaming goroutine.
	startPolling := func() {
		pollMu.Lock()
		if pollCancel != nil {
			pollCancel()
		}
		pollCtx, pCancel := context.WithCancel(ctx)
		pollCancel = pCancel
		pollMu.Unlock()

		go func() {
			for {
				for _, group := range register.SystemGroups {
					for _, probe := range group.Probes {
						// Check for cancellation between each read
						select {
						case <-pollCtx.Done():
							return
						default:
						}

						data, err := b.ReadRegisters(pollCtx, probe.Addr, probe.Count)
						if err != nil {
							if pollCtx.Err() != nil {
								return // context cancelled, stop quietly
							}
							lbl := valueLabels[probe.Addr]
							if lbl != nil {
								if isTimeRegister(probe.Addr) {
									// Don't overwrite time label on individual register errors
									continue
								}
								lbl.SetText(fmt.Sprintf("error: %v", err))
							}
							continue
						}

						if isTimeRegister(probe.Addr) {
							// Accumulate time register value
							if len(data) >= 2 {
								val := binary.BigEndian.Uint16(data[:2])
								timeMu.Lock()
								timeVals[probe.Addr] = val
								// Update composite time only when all 6 values are present
								if len(timeVals) == 6 {
									composed := register.ComposeSystemTime(
										timeVals[addrTimeYear],
										timeVals[addrTimeMonth],
										timeVals[addrTimeDay],
										timeVals[addrTimeHour],
										timeVals[addrTimeMin],
										timeVals[addrTimeSec],
									)
									timeLabel.SetText(composed)
								}
								timeMu.Unlock()
							}
							continue
						}

						formatted := register.FormatValue(probe, data)
						lbl := valueLabels[probe.Addr]
						if lbl != nil {
							lbl.SetText(formatted)
						}
					}
				}
				// Full cycle done, loop back for continuous polling
			}
		}()
	}

	// stopPolling cancels the streaming goroutine and resets values.
	stopPolling := func() {
		pollMu.Lock()
		if pollCancel != nil {
			pollCancel()
			pollCancel = nil
		}
		pollMu.Unlock()
		resetAllValues()
	}

	// === Connect/Disconnect button logic ===

	connected := false
	connectBtn.OnTapped = func() {
		if !connected {
			connectBtn.Disable()
			go func() {
				err := b.Reconfigure(ctx, inverterAddr, byte(*slaveID))
				if err != nil {
					logger.Error("connection failed", "error", err)
				}
			}()
		} else {
			connectBtn.Disable()
			stopPolling()
			go func() {
				err := b.Disconnect(ctx)
				if err != nil {
					logger.Error("disconnect failed", "error", err)
				}
			}()
		}
	}

	// === State event listener ===

	go func() {
		events := b.StateEvents()
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-events:
				if !ok {
					return
				}
				stateStr := evt.State.String()
				statusLabel.SetText(stateStr)
				statusIndicator.FillColor = stateColor(evt.State)
				statusIndicator.Refresh()

				switch evt.State {
				case broker.StateConnected:
					connected = true
					connectBtn.SetText("Disconnect")
					connectBtn.Enable()
					startPolling()
				case broker.StateDisconnected, broker.StateDormant:
					connected = false
					connectBtn.SetText("Connect")
					connectBtn.Enable()
					stopPolling()
				case broker.StateConnecting, broker.StateReconnecting:
					connectBtn.Disable()
				}
			}
		}
	}()

	// === Layout ===

	indicatorContainer := container.New(layout.NewCenterLayout(),
		container.New(layout.NewCustomPaddedLayout(0, 0, 4, 4), statusIndicator),
	)
	connectionBar := container.NewHBox(
		indicatorContainer,
		layout.NewSpacer(),
		statusLabel,
		layout.NewSpacer(),
		connectBtn,
	)

	topBar := container.NewVBox(
		connectionBar,
		widget.NewSeparator(),
	)

	mainContent := container.NewBorder(topBar, nil, nil, nil, scrollContent)
	win.SetContent(mainContent)

	// Handle OS signals for graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-sigCh:
			cancel()
			b.Close()
			fyneApp.Quit()
		case <-ctx.Done():
		}
	}()

	// Handle window close
	win.SetOnClosed(func() {
		stopPolling()
		cancel()
		b.Close()
	})

	// Print startup info
	logger.Info("fyne PoC started",
		"inverter", inverterAddr,
		"mode", *modbusMode,
		"slaveID", *slaveID,
	)

	// Run the Fyne event loop (blocks until window closes)
	win.ShowAndRun()

	// Ensure cleanup after Fyne exits
	cancel()
	b.Close()

	_ = theme.DefaultTheme() // suppress unused import warning if needed
}
