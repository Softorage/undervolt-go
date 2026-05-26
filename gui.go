//go:build gui

// gui.go

/*
Build GUI version

go build -tags gui -o undervolt-go-pro
go build -tags gui -ldflags="-X main.version=$(git describe --tags)" -o undervolt-go-pro

This will include gui.go, and runGUI() will launch your Fyne GUI.
*/

package main

import (
	"bytes"
	"fmt"
	"image/color"
	"os"
	"os/exec"

	"net/url"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/spf13/viper"
)

// Used in main.go by rootCmd
const rootCmdUseString = "undervolt-go-pro"

// ---------------------------------------------------------------------
// CUSTOM WIDGETS
// ---------------------------------------------------------------------

type infoEntry struct {
	widget.Entry
	infoMsg  string
	showInfo func(string, time.Duration)
}

func newInfoEntry(msg string, show func(string, time.Duration)) *infoEntry {
	e := &infoEntry{infoMsg: msg, showInfo: show}
	e.ExtendBaseWidget(e)
	return e
}

func (e *infoEntry) FocusGained() {
	e.Entry.FocusGained()
	if e.infoMsg != "" && e.showInfo != nil {
		e.showInfo(e.infoMsg, 6*time.Second)
	}
}

type infoCheck struct {
	widget.Check
	infoMsg  string
	showInfo func(string, time.Duration)
}

func newInfoCheck(label, msg string, show func(string, time.Duration)) *infoCheck {
	c := &infoCheck{infoMsg: msg, showInfo: show}
	c.Text = label
	c.ExtendBaseWidget(c)
	return c
}

func (c *infoCheck) Tapped(pe *fyne.PointEvent) {
	c.Check.Tapped(pe)
	if c.infoMsg != "" && c.showInfo != nil {
		c.showInfo(c.infoMsg, 6*time.Second)
	}
}

type infoSelect struct {
	widget.Select
	infoMsg  string
	showInfo func(string, time.Duration)
}

func newInfoSelect(options []string, msg string, show func(string, time.Duration)) *infoSelect {
	s := &infoSelect{infoMsg: msg, showInfo: show}
	s.Options = options
	s.ExtendBaseWidget(s)
	return s
}

func (s *infoSelect) Tapped(pe *fyne.PointEvent) {
	s.Select.Tapped(pe)
	if s.infoMsg != "" && s.showInfo != nil {
		s.showInfo(s.infoMsg, 6*time.Second)
	}
}

// Voltage Offset Inputs (with enable checkboxes)
type planeUI struct {
	name    string
	command string
	entry   *infoEntry
	check   *infoCheck
}

// ---------------------------------------------------------------------
// APP GUI STRUCT
// ---------------------------------------------------------------------

// AppGUI holds the state, data bindings, and widgets for the application
type AppGUI struct {
	app    fyne.App
	window fyne.Window

	// Data Bindings & Timers
	// stores output to be shown on the Output Pane
	// To make Fyne GUI app goroutine-safe and allow automatic widget updates (like outputLabel) from a background goroutine, use the data/binding package instead of directly calling SetText() on the label from another goroutine
	outputLabelBinding binding.String
	outputWarningBind  binding.String
	warningTimer       *time.Timer

	// Output Label: Status tab output
	// Output Warning: Bottom info bar
	outputLabel   *widget.Label
	outputWarning *widget.Label

	// Core UI Elements
	planes       []planeUI
	p1Power      *infoEntry
	p1Time       *infoEntry
	p2Power      *infoEntry
	p2Time       *infoEntry
	tempEntry    *infoEntry
	tempBatEntry *infoEntry
	forceCheck   *infoCheck
	lockCheck    *infoCheck
	verboseCheck *infoCheck
	turboSelect  *infoSelect
	turboOptions map[string]string
	persistCheck *widget.Check
	persistInfo  *widget.Label

	// Profile Selects
	profileSaveSelect *widget.Select
	profileLoadSelect *widget.Select

	// Monitoring State
	monitorTicker *time.Ticker
	stopMonitor   chan struct{}
}

// ---------------------------------------------------------------------
// ENTRY POINT
// ---------------------------------------------------------------------

func runGUI() {
	a := app.NewWithID("com.softorage.undervolt-go")
	a.Settings().SetTheme(theme.DarkTheme())
	
	a.SetIcon(resourceIconPng)

	w := a.NewWindow("Undervolt Go")
	w.Resize(fyne.NewSize(800, 600))

	gui := &AppGUI{
		app:    a,
		window: w,
	}

	gui.initWidgets() // Safely initialize widgets AFTER app creation
	gui.buildLayout() // Assemble the UI

	w.ShowAndRun()
}

// ---------------------------------------------------------------------
// WIDGET INITIALIZATION
// ---------------------------------------------------------------------

func (g *AppGUI) initWidgets() {
	// Output & Warnings
	g.outputLabelBinding = binding.NewString()
	g.outputLabelBinding.Set("Click 'Read' to view current settings.")
	g.outputLabel = widget.NewLabelWithData(g.outputLabelBinding)
	g.outputLabel.Wrapping = fyne.TextWrapWord

	g.outputWarningBind = binding.NewString()
	g.outputWarning = widget.NewLabelWithData(g.outputWarningBind)
	g.outputWarning.Wrapping = fyne.TextWrapWord

	// Planes
	g.planes = []planeUI{
		{"Core", "core", newInfoEntry("Voltage offset for Core plane (e.g., -50.000 mV)", g.showWarning), newInfoCheck("", "Enable undervolt for Core plane", g.showWarning)},
		{"Cache", "cache", newInfoEntry("Voltage offset for Cache plane (e.g., -50.000 mV)", g.showWarning), newInfoCheck("", "Enable undervolt for Cache plane", g.showWarning)},
		{"GPU", "gpu", newInfoEntry("Voltage offset for GPU plane (e.g., -50.000 mV)", g.showWarning), newInfoCheck("", "Enable undervolt for GPU plane", g.showWarning)},
		{"Uncore", "uncore", newInfoEntry("Voltage offset for Uncore plane (e.g., -50.000 mV)", g.showWarning), newInfoCheck("", "Enable undervolt for Uncore plane", g.showWarning)},
		{"AnalogIO", "analogio", newInfoEntry("Voltage offset for AnalogIO plane (e.g., -50.000 mV)", g.showWarning), newInfoCheck("", "Enable undervolt for AnalogIO plane", g.showWarning)},
	}
	
	floatValidator := func(s string) error {
		if s == "" {
			return nil
		}
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("must be a float")
		}
		return nil
	}
	for _, p := range g.planes {
		p.entry.SetPlaceHolder("e.g. -50.000")
		p.entry.Validator = floatValidator
	}

	// Power Limits
	g.p1Power = newInfoEntry("P1 power limit in watts (e.g., 45) is the long term power limit, that can be safe for longer periods.", g.showWarning)
	g.p1Power.SetPlaceHolder("Power (W)")
	g.p1Time = newInfoEntry("Time window for P1 in seconds (e.g., 28).", g.showWarning)
	g.p1Time.SetPlaceHolder("Time (s)")
	g.p2Power = newInfoEntry("P2 power limit in watts (e.g., 60) is the short term power limit, that can be safe for shorter periods and is useful for short bursts of performance.", g.showWarning)
	g.p2Power.SetPlaceHolder("Power (W)")
	g.p2Time = newInfoEntry("Time window for P2 in seconds (e.g., 2).", g.showWarning)
	g.p2Time.SetPlaceHolder("Time (s)")

	intValidator := func(s string) error {
		if s == "" {
			return nil
		}
		if _, err := strconv.Atoi(s); err != nil {
			return fmt.Errorf("must be integer")
		}
		return nil
	}
	for _, e := range []*infoEntry{g.p1Power, g.p1Time, g.p2Power, g.p2Time} {
		e.Validator = intValidator
	}

	// Other Flags
	g.forceCheck = newInfoCheck("Force positive voltage offsets", "Force writing positive voltage offsets (useful for overclocking). Be very careful. This is danger zone and may permanantly damage your CPU or other components if you don't 100% know what you are doing.", g.showWarning)
	g.lockCheck = newInfoCheck("Lock power limit", "Lock the power limit settings to prevent changes.", g.showWarning)
	g.verboseCheck = newInfoCheck("Enable Verbose Output", "Enable detailed output from undervolt-go.", g.showWarning)

	g.turboOptions = map[string]string{
		"Default":  "-1",
		"Enabled":  "0",
		"Disabled": "1",
	}
	g.turboSelect = newInfoSelect([]string{"Default", "Enabled", "Disabled"}, "Control turbo mode: Default (No change), Enabled (Allow turbo), Disabled (Block turbo).", g.showWarning)

	// Temperature Limits
	g.tempEntry = newInfoEntry("Maximum temperature on AC power (°C).", g.showWarning)
	g.tempEntry.SetPlaceHolder("AC °C")
	g.tempEntry.Validator = intValidator

	g.tempBatEntry = newInfoEntry("Maximum temperature on battery (°C).", g.showWarning)
	g.tempBatEntry.SetPlaceHolder("Battery °C")
	g.tempBatEntry.Validator = intValidator

	// Settings
	g.persistCheck = widget.NewCheck("Persist the current undervolt configuration across reboots.", nil)
	g.persistInfo = widget.NewLabel("Make sure the configuration being persisted is indeed a stable one. Untick this checkbox when not needed. You need to check and hit 'Apply' to persist the current values.")
	g.persistInfo.Wrapping = fyne.TextWrapWord

	// 7. Profiles
	g.profileSaveSelect = widget.NewSelect([]string{"AC", "Battery"}, nil)
	g.profileLoadSelect = widget.NewSelect([]string{"Auto", "AC", "Battery"}, nil)
}

// ---------------------------------------------------------------------
// HELPER METHODS
// ---------------------------------------------------------------------

// Centralized warning handler that prevents premature clearing of messages
func (g *AppGUI) showWarning(msg string, duration time.Duration) {
	g.outputWarningBind.Set(msg)
	if g.warningTimer != nil {
		g.warningTimer.Stop()
	}
	if duration > 0 {
		g.warningTimer = time.AfterFunc(duration, func() {
			g.outputWarningBind.Set("")
		})
	}
}
// Collect flags from input elements and return
func (g *AppGUI) collect() []string {
	var args []string
	var outputLabelAlertArray []string
	
	for _, p := range g.planes {
		if p.check.Checked {
			args = append(args, "--"+p.command+"="+p.entry.Text)
		} else {
			outputLabelAlertArray = append(outputLabelAlertArray, "Voltage offset for "+p.name+" was not applied as the corresponding checkbox is unchecked.\n")
		}
	}
	if len(outputLabelAlertArray) > 0 {
		g.outputLabelBinding.Set(strings.Join(outputLabelAlertArray, "\n"))
		g.showWarning("Error occured when applying Voltage Offset settings. Please check 'Output' pane for more information.", 3*time.Second)
	}

	if g.forceCheck.Checked {
		args = append(args, "--force")
	}
	if g.lockCheck.Checked {
		args = append(args, "--lock-power-limit")
	}
	if g.verboseCheck.Checked {
		args = append(args, "--verbose")
	}
	if val, ok := g.turboOptions[g.turboSelect.Selected]; ok {
		args = append(args, "--turbo="+val)
	}
	if g.tempEntry.Text != "" {
		args = append(args, "--temp="+g.tempEntry.Text)
	}
	if g.tempBatEntry.Text != "" {
		args = append(args, "--temp-bat="+g.tempBatEntry.Text)
	}
	if g.p1Power.Text != "" && g.p1Time.Text != "" {
		args = append(args, "--p1="+g.p1Power.Text+","+g.p1Time.Text)
	}
	if g.p2Power.Text != "" && g.p2Time.Text != "" {
		args = append(args, "--p2="+g.p2Power.Text+","+g.p2Time.Text)
	}
	if g.persistCheck.Checked {
		args = append(args, "--persist")
	}
	return args
}

func (g *AppGUI) run(flags ...string) error {
	cmd := exec.Command("sudo", append([]string{rootCmdUseString}, flags...)...)
	// Redirect command output to a buffer for display in the Output Pane.
	// Redirect both stdout and stderr to the same buffer, so that any error
	// messages are included in the output.
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err != nil {
		buf.WriteString("\nError: " + err.Error())
		g.showWarning("Error occured when applying settings. Please check 'Output' pane for more information.", 3*time.Second)
	}
	g.outputLabelBinding.Set(buf.String())
	return err
}
// startMonitor spins up a goroutine that every second runs `command`
// and dumps its stdout/stderr into outputLabel (including any errors).
func (g *AppGUI) startMonitor(command string) {
	if g.monitorTicker != nil {
		return // already running
	}
	g.stopMonitor = make(chan struct{})
	g.monitorTicker = time.NewTicker(1 * time.Second)
	// Pass channels into the goroutine as arguments so it holds steady references
	// even if the outer variables are set to nil by the UI thread.
	go func(stop chan struct{}, ticker *time.Ticker) {
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				// run via shell so pipes/greps work
				cmd := exec.Command("sh", "-c", command)
				var buf bytes.Buffer
				cmd.Stdout = &buf
				cmd.Stderr = &buf
				if err := cmd.Run(); err != nil {
					buf.WriteString("\nError: " + err.Error())
				}
				g.outputLabelBinding.Set(buf.String())
			}
		}
	}(g.stopMonitor, g.monitorTicker)
}
// stopMonitorFunc tells that goroutine to exit
func (g *AppGUI) stopMonitorFunc() {
	if g.stopMonitor != nil {
		close(g.stopMonitor)
		g.stopMonitor = nil
	}
	// Clear the ticker state purely on the main UI thread to avoid data races
	if g.monitorTicker != nil {
		g.monitorTicker.Stop()
		g.monitorTicker = nil
	}
}

// ---------------------------------------------------------------------
// TAB BUILDERS
// ---------------------------------------------------------------------

func (g *AppGUI) buildVoltageOffsetTab() fyne.CanvasObject {
	voltOffsetCont := container.New(layout.NewFormLayout())
	for _, p := range g.planes {
		voltOffsetCont.Add(container.New(layout.NewHBoxLayout(), p.check, widget.NewLabel(p.name)))
		voltOffsetCont.Add(p.entry)
	}
	return container.NewPadded(
		container.NewVBox(
			widget.NewRichTextFromMarkdown("## Voltage Offset"),
			widget.NewSeparator(),
			voltOffsetCont,
		),
	)
}

func (g *AppGUI) buildPowerLimitTab() fyne.CanvasObject {
	plForm := container.New(layout.NewFormLayout(),
		widget.NewLabel("P1 Power (W)"), g.p1Power,
		widget.NewLabel("P1 Time (s)"), g.p1Time,
		widget.NewLabel("P2 Power (W)"), g.p2Power,
		widget.NewLabel("P2 Time (s)"), g.p2Time,
	)
	return container.NewPadded(
		container.NewVBox(
			widget.NewRichTextFromMarkdown("## Power Limit"),
			widget.NewSeparator(),
			plForm,
		),
	)
}

func (g *AppGUI) buildTempLimitTab() fyne.CanvasObject {
	tempGrid := container.New(layout.NewFormLayout(),
		widget.NewLabel("AC Temp (°C)"), g.tempEntry,
		widget.NewLabel("Battery Temp (°C)"), g.tempBatEntry,
	)
	return container.NewPadded(
		container.NewVBox(
			widget.NewRichTextFromMarkdown("## Temperature Limit"),
			widget.NewSeparator(),
			tempGrid,
		),
	)
}

func (g *AppGUI) buildOtherFlagsTab() fyne.CanvasObject {
	return container.NewPadded(
		container.NewVBox(
			widget.NewRichTextFromMarkdown("## Other Flags"),
			widget.NewSeparator(),
			g.forceCheck,
			g.lockCheck,
			g.verboseCheck,
			container.NewHBox(),
			container.New(layout.NewFormLayout(), widget.NewLabel("Turbo"), g.turboSelect),
		),
	)
}

func (g *AppGUI) buildProfilesTab() fyne.CanvasObject {
	profileSaveBtn := widget.NewButton("Save Profile", func() {
		name := g.profileSaveSelect.Selected
		if name == "" {
			g.showWarning("Please select a profile to save.", 3*time.Second)
			return
		}
		args := g.collect()
		flags := append([]string{"profile", "save", strings.ToLower(name)}, args...)
		if len(args) > 0 {
			if err := g.run(flags...); err == nil {
				g.showWarning("Settings saved successfully as profile "+name+".", 3*time.Second)
			}
		}
	})

	profileLoadBtn := widget.NewButton("Load Profile", func() {
		name := g.profileLoadSelect.Selected
		if name == "" {
			g.showWarning("Please select a profile to load.", 3*time.Second)
			return
		}

		actualName := name
		if actualName == "Auto" {
			// isBatteryDischarging is available main.go
			if isBatteryDischarging() {
				actualName = "Battery"
			} else {
				actualName = "AC"
			}
		}

		// Reload config from disk to catch updated profiles
		// initConfig is available in package main.go
		initConfig()

		key := "profiles." + strings.ToLower(actualName)
		if !viper.IsSet(key) {
			g.showWarning(fmt.Sprintf("Profile '%s' not found.", actualName), 3*time.Second)
			return
		}

		// Get the values from the profile
		p := viper.Sub(key)
		coreOffset := p.GetFloat64("planes.core")
		gpuOffset := p.GetFloat64("planes.gpu")
		cacheOffset := p.GetFloat64("planes.cache")
		uncoreOffset := p.GetFloat64("planes.uncore")
		analogioOffset := p.GetFloat64("planes.analogio")
		tempFlag := p.GetInt("tl.temp")
		tempBatFlag := p.GetInt("tl.temp-bat")
		turboFlag := p.GetInt("turbo")
		// we use p.GetIntSlice here as the values are int. we couldn't in main.go as the flags were string.
		p1Args := p.GetIntSlice("pl.p1")
		p2Args := p.GetIntSlice("pl.p2")
		/* below code is useful if the values of p1 or p2 array float ... keep it
		if raw := p.Get("pl.p1"); raw != nil {
			if arr, ok := raw.([]any); ok && len(arr) == 2 {
				p1Args = []string{fmt.Sprint(arr[0]), fmt.Sprint(arr[1])}
			}
		}
		if raw := p.Get("pl.p2"); raw != nil {
			if arr, ok := raw.([]any); ok && len(arr) == 2 {
				p2Args = []string{fmt.Sprint(arr[0]), fmt.Sprint(arr[1])}
			}
		}
		*/

		// Update the values in the entry widgets
		for _, plane := range g.planes {
			switch plane.name {
			case "Core":
				plane.entry.SetText(fmt.Sprintf("%f", coreOffset))
			case "Cache":
				plane.entry.SetText(fmt.Sprintf("%f", cacheOffset))
			case "GPU":
				plane.entry.SetText(fmt.Sprintf("%f", gpuOffset))
			case "Uncore":
				plane.entry.SetText(fmt.Sprintf("%f", uncoreOffset))
			case "AnalogIO":
				plane.entry.SetText(fmt.Sprintf("%f", analogioOffset))
			}
		}
		
		if len(p1Args) == 2 {
			g.p1Power.SetText(fmt.Sprintf("%d", p1Args[0]))
			g.p1Time.SetText(fmt.Sprintf("%d", p1Args[1]))
		} else {
			g.p1Power.SetText("")
			g.p1Time.SetText("")
		}
		
		if len(p2Args) == 2 {
			g.p2Power.SetText(fmt.Sprintf("%d", p2Args[0]))
			g.p2Time.SetText(fmt.Sprintf("%d", p2Args[1]))
		} else {
			g.p2Power.SetText("")
			g.p2Time.SetText("")
		}

		g.tempEntry.SetText(fmt.Sprintf("%d", tempFlag))
		g.tempBatEntry.SetText(fmt.Sprintf("%d", tempBatFlag))

		turboProfile := ""
		for option, value := range g.turboOptions {
			if value == strconv.Itoa(turboFlag) {
				turboProfile = option
				break
			}
		}
		if turboProfile != "" {
			g.turboSelect.SetSelected(turboProfile)
		} else {
			g.turboSelect.ClearSelected()
		}

		g.showWarning(fmt.Sprintf("Profile '%s' loaded into the UI.", actualName), 3*time.Second)
	})

	isAutoSwitchEnabled := func() bool {
		_, err := os.Stat("/etc/udev/rules.d/99-undervolt-go-auto.rules")
		return err == nil
	}

	autoSwitchLabel := widget.NewLabel("Enable automatic profile switching based on whether the battery is charging or discharging. Make sure that both AC and Battery profiles exist before enabling.")
	autoSwitchLabel.Wrapping = fyne.TextWrapWord

	autoSwitchBtn := widget.NewButton("", nil)
	updateAutoSwitchBtn := func() {
		if isAutoSwitchEnabled() {
			autoSwitchBtn.SetText("Click to disable")
		} else {
			autoSwitchBtn.SetText("Click to enable")
		}
	}
	updateAutoSwitchBtn()

	autoSwitchBtn.OnTapped = func() {
		// Make sure we have the latest config state
		initConfig()

		if !isAutoSwitchEnabled() {
			// Check if profiles exist before allowing it to be enabled
			if !viper.IsSet("profiles.ac") || !viper.IsSet("profiles.battery") {
				g.showWarning("Both 'AC' and 'Battery' profiles must exist before enabling auto-profile switching.", 4*time.Second)
				return
			}
			if err := g.run("profile", "auto-switch", "enable"); err == nil {
				g.showWarning("Auto profile switching enabled.", 3*time.Second)
			}
		} else {
			if err := g.run("profile", "auto-switch", "disable"); err == nil {
				g.showWarning("Auto profile switching disabled.", 3*time.Second)
			}
		}
		updateAutoSwitchBtn()
	}

	return container.NewPadded(
		container.NewVBox(
			widget.NewRichTextFromMarkdown("## Profiles"),
			widget.NewSeparator(),
			widget.NewLabel("Save current settings to a profile:"),
			g.profileSaveSelect,
			profileSaveBtn,
			widget.NewLabel(""),
			widget.NewLabel("Load settings from a profile:"),
			g.profileLoadSelect,
			profileLoadBtn,
			widget.NewLabel(""),
			widget.NewSeparator(),
			autoSwitchLabel,
			autoSwitchBtn,
		),
	)
}

func (g *AppGUI) buildSettingsTab() fyne.CanvasObject {
	clearPersistBtn := widget.NewButton("Clear persisted configuration", func() {
		if err := g.run("--disable-persist"); err == nil {
			g.showWarning("Persisted configuration cleared successfully.", 3*time.Second)
		}
	})
	return container.NewPadded(
		container.NewVBox(
			widget.NewRichTextFromMarkdown("## Settings"),
			widget.NewSeparator(),
			g.persistCheck,
			g.persistInfo,
			widget.NewLabel(""),
			clearPersistBtn,
		),
	)
}

func (g *AppGUI) buildStatusTab() fyne.CanvasObject {
	readBtn := widget.NewButton("Read", func() {
		if g.monitorTicker != nil {
			g.showWarning("Please click 'Stop' before running this command.", 3*time.Second)
			return
		}
		_ = g.run("--read")
	})
	helpBtn := widget.NewButton("Help", func() {
		if g.monitorTicker != nil {
			g.showWarning("Please click 'Stop' before running this command.", 3*time.Second)
			return
		}
		_ = g.run("--help")
	})
	checkTempsBtn := widget.NewButton("Check Temps", func() {
		if g.monitorTicker != nil {
			g.showWarning("Please click 'Stop' before running this command.", 3*time.Second)
			return
		}
		g.showWarning("Please click 'Stop' before running any other command or closing the app.", 3*time.Second)
		g.startMonitor("sensors | grep Core")
	})
	checkFansBtn := widget.NewButton("Check Fans", func() {
		if g.monitorTicker != nil {
			g.showWarning("Please click 'Stop' before running this command.", 3*time.Second)
			return
		}
		g.showWarning("Please click 'Stop' before running any other command or closing the app.", 3*time.Second)
		g.startMonitor("sensors | grep -e cpu_fan -e gpu_fan")
	})
	stopBtn := widget.NewButton("Stop", func() {
		if g.monitorTicker == nil {
			g.showWarning("Nothing to stop there. You're good. :)", 3*time.Second)
			return
		}
		g.stopMonitorFunc()
		g.showWarning("Monitoring stopped. You may now run other commands.", 3*time.Second)
	})
	verBtn := widget.NewButton("Version", func() {
		if g.monitorTicker != nil {
			g.showWarning("Please click 'Stop' before running this command.", 3*time.Second)
			return
		}
		_ = g.run("--version")
	})

	btnBar := container.NewHBox(
		checkTempsBtn, // lets separate the monitoring buttons into a new section named monitor. there user only has two buttons start and stop. star shows all the info, core temps, fans rpm, core freq, cpu utilization .... also we can have an information section which shows information about the cpu, ram, and stuff
		checkFansBtn,
		stopBtn,
		layout.NewSpacer(),
		readBtn,
		helpBtn,
		verBtn,
	)
	
	return container.NewPadded(
		container.NewBorder(
			container.NewVBox(
				widget.NewRichTextFromMarkdown("## Status"),
				widget.NewSeparator(),
			),
			btnBar,
			nil, nil,
			g.outputLabel, 
		),
	)
}

// ---------------------------------------------------------------------
// MAIN LAYOUT ASSEMBLY
// ---------------------------------------------------------------------

func (g *AppGUI) buildLayout() {
	voltageOffsetTab := g.buildVoltageOffsetTab()
	powerLimitTab := g.buildPowerLimitTab()
	tempLimitTab := g.buildTempLimitTab()
	otherFlagsTab := g.buildOtherFlagsTab()
	profilesTab := g.buildProfilesTab()
	settingsTab := g.buildSettingsTab()
	statusTab := g.buildStatusTab()

	// Create a Max container that will act as the dynamic main content area
	contentArea := container.NewMax()

	secNames := []string{"Voltage Offset", "Power Limit", "Temperature Limits", "Other Flags", "Profiles", "Settings", "Status"}

	tabs := widget.NewList(
		func() int { return len(secNames) },
		func() fyne.CanvasObject {
			lbl := widget.NewLabel("")
			lbl.TextStyle = fyne.TextStyle{Bold: true}
			return container.NewPadded(lbl)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			// Updating list elements safely
			o.(*fyne.Container).Objects[0].(*widget.Label).SetText(secNames[i])
		},
	)

	tabs.OnSelected = func(id widget.ListItemID) {
		switch id {
		case 0:
			contentArea.Objects = []fyne.CanvasObject{voltageOffsetTab}
		case 1:
			contentArea.Objects = []fyne.CanvasObject{powerLimitTab}
		case 2:
			contentArea.Objects = []fyne.CanvasObject{tempLimitTab}
		case 3:
			contentArea.Objects = []fyne.CanvasObject{otherFlagsTab}
		case 4:
			contentArea.Objects = []fyne.CanvasObject{profilesTab}
		case 5:
			contentArea.Objects = []fyne.CanvasObject{settingsTab}
		case 6:
			contentArea.Objects = []fyne.CanvasObject{statusTab}
		}
		contentArea.Refresh()
	}

	// Construct Sidebar Elements
	// Top: App Name and version
	appLabel := widget.NewRichText(
		&widget.TextSegment{
			Text: "Undervolt Go",
			Style: widget.RichTextStyle{
				SizeName:  theme.SizeNameHeadingText,
				TextStyle: fyne.TextStyle{Bold: true},
				Alignment: fyne.TextAlignCenter,
			},
		},
		&widget.TextSegment{
			Text: fmt.Sprintf("\nversion %s", version), // version is defined in main.go
			Style: widget.RichTextStyle{
				SizeName:  theme.SizeNameCaptionText,
				ColorName: theme.ColorNamePlaceHolder,
				Alignment: fyne.TextAlignCenter,
			},
		},
	)

	centeredAppLabel := container.NewHBox(layout.NewSpacer(), appLabel, layout.NewSpacer())

	sidebarTop := container.NewVBox(
		container.NewPadded(centeredAppLabel),
		widget.NewLabel(""),
	)

	// Bottom: Github URL, Sponsor URL & Version
	sourceCodeURL, _ := url.Parse("https://github.com/Softorage/7z-GUI-Linux")
	sponsorURL, _ := url.Parse("https://rzp.io/rzp/hY39lZGa")

	//sourceCodeBtn := widget.NewButtonWithIcon("View Source", resourceSourceCodeSvg, func() { a.OpenURL(sourceCodeURL) })
	sourceCodeBtn := widget.NewButton("View Source", func() { g.app.OpenURL(sourceCodeURL) })
	//sourceCodeBtn.IconPlacement = widget.ButtonIconLeadingText
	sourceCodeBtn.Importance = widget.LowImportance
	sourceCodeBtn.Alignment = widget.ButtonAlignLeading

	sponsorBtn := widget.NewButton("Sponsor", func() { g.app.OpenURL(sponsorURL) })
	sponsorBtn.Importance = widget.LowImportance
	sponsorBtn.Alignment = widget.ButtonAlignLeading

	tabsBottom := container.NewVBox(
		container.NewPadded(sourceCodeBtn),
		container.NewPadded(sponsorBtn),
	)

	aboutText := widget.NewRichText(&widget.TextSegment{
		Text: "A Softorage Project",
		Style: widget.RichTextStyle{
			SizeName:  theme.SizeNameCaptionText,
			ColorName: theme.ColorNamePlaceHolder,
		},
	})

	sidebarBottom := container.NewVBox(
		tabsBottom,
		container.NewCenter(aboutText),
	)

	sidebarContent := container.NewBorder(sidebarTop, sidebarBottom, nil, nil, tabs)

	// Create Sidebar Background
	// A translucent gray (alpha=25) creates a subtle contrast for both Light and Dark themes.
	sidebarBg := canvas.NewRectangle(color.NRGBA{R: 128, G: 128, B: 128, A: 25})
	// Force a minimum width to make the sidebar cozier/wider (180px width)
	sidebarBg.SetMinSize(fyne.NewSize(180, 0))
	// Combine the background color and the sidebar content
	sidebar := container.NewMax(sidebarBg, sidebarContent)

	// Action Buttons (docked bottom‑right)
	settingsApplyBtn := widget.NewButton("Apply", func() {
		// without len(args) > 0, clicking on apply without any setting relaunches another window of Undervolt Go
		args := g.collect()
		if len(args) > 0 {
			if err := g.run(args...); err == nil {
				g.showWarning("Settings applied successfully.", 3*time.Second)
			}
		}
	})
	mainBtnBar := container.NewHBox(layout.NewSpacer(), settingsApplyBtn)

	mainLayout := container.NewBorder(
		nil,
		nil,
		sidebar,
		nil,
		container.NewBorder(
			nil,
			container.NewVBox(widget.NewSeparator(), mainBtnBar, g.outputWarning),
			nil,
			nil,
			contentArea,
		),
	)

	g.window.SetContent(mainLayout)

	// Default to first tab
	tabs.Select(0)
}