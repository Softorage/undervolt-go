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
	"os" // required for os.Stat ... may also be required if code for allowing notification for non-sudo user is uncommented
	"os/exec"

	"net/url"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/spf13/viper"
)

// Used in main.go by rootCmd
const rootCmdUseString = "undervolt-go-pro"

// Custom Widgets for Showing Info on Interaction
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

func newInfoSelect(options[]string, msg string, show func(string, time.Duration)) *infoSelect {
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

func runGUI() {
	a := app.NewWithID("com.softorage.undervolt-go")
	// Set dark theme
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("Undervolt Go")
	w.Resize(fyne.NewSize(800, 600))

	// stores output to be shown on the Output Pane
	// To make Fyne GUI app goroutine-safe and allow automatic widget updates (like outputLabel) from a background goroutine, use the data/binding package instead of directly calling SetText() on the label from another goroutine
	outputLabelBinding := binding.NewString()
	outputLabelBinding.Set("Click 'Read' to view current settings.")
	outputLabel := widget.NewLabelWithData(outputLabelBinding)
	outputLabel.Wrapping = fyne.TextWrapWord // wrap long lines

	// Output Pane (read-only)
	outputWarningBind := binding.NewString()
	outputWarning := widget.NewLabelWithData(outputWarningBind)
	outputWarning.Wrapping = fyne.TextWrapWord // wrap tooltip texts preventing UI jumping

	// Centralized warning handler that prevents premature clearing of messages
	var warningTimer *time.Timer
	showWarning := func(msg string, duration time.Duration) {
		outputWarningBind.Set(msg)
		if warningTimer != nil {
			warningTimer.Stop()
		}
		if duration > 0 {
			warningTimer = time.AfterFunc(duration, func() {
				outputWarningBind.Set("")
			})
		}
	}

	// Voltage Offset Inputs (with enable checkboxes)
	type planeUI struct {
		name    string
		command string
		entry   *infoEntry
		check   *infoCheck
	}
	planes :=[]planeUI{
		{"Core", "core", newInfoEntry("Voltage offset for Core plane (e.g., -50.000 mV)", showWarning), newInfoCheck("", "Enable undervolt for Core plane", showWarning)},
		{"Cache", "cache", newInfoEntry("Voltage offset for Cache plane (e.g., -50.000 mV)", showWarning), newInfoCheck("", "Enable undervolt for Cache plane", showWarning)},
		{"GPU", "gpu", newInfoEntry("Voltage offset for GPU plane (e.g., -50.000 mV)", showWarning), newInfoCheck("", "Enable undervolt for GPU plane", showWarning)},
		{"Uncore", "uncore", newInfoEntry("Voltage offset for Uncore plane (e.g., -50.000 mV)", showWarning), newInfoCheck("", "Enable undervolt for Uncore plane", showWarning)},
		{"AnalogIO", "analogio", newInfoEntry("Voltage offset for AnalogIO plane (e.g., -50.000 mV)", showWarning), newInfoCheck("", "Enable undervolt for AnalogIO plane", showWarning)},
	}
	for _, p := range planes {
		p.entry.SetPlaceHolder("e.g. -50.000")
		p.entry.Validator = func(s string) error {
			if s == "" {
				return nil
			}
			if _, err := strconv.ParseFloat(s, 64); err != nil {
				return fmt.Errorf("must be a float")
			}
			return nil
		}
	}

	// Power Limit Inputs
	p1Power := newInfoEntry("P1 power limit in watts (e.g., 45) is the long term power limit, that can be safe for longer periods.", showWarning)
	p1Power.SetPlaceHolder("Power (W)")

	p1Time := newInfoEntry("Time window for P1 in seconds (e.g., 28).", showWarning)
	p1Time.SetPlaceHolder("Time (s)")

	p2Power := newInfoEntry("P2 power limit in watts (e.g., 60) is the short term power limit, that can be safe for shorter periods and is useful for short bursts of performance.", showWarning)
	p2Power.SetPlaceHolder("Power (W)")

	p2Time := newInfoEntry("Time window for P2 in seconds (e.g., 2).", showWarning)
	p2Time.SetPlaceHolder("Time (s)")

	// Validate whether the input is an integer
	intValidator := func(s string) error {
		if s == "" {
			return nil
		}
		if _, err := strconv.Atoi(s); err != nil {
			return fmt.Errorf("must be integer")
		}
		return nil
	}
	for _, e := range []*infoEntry{p1Power, p1Time, p2Power, p2Time} {
		e.Validator = intValidator
	}

	// Other Flags
	forceCheck := newInfoCheck("Force positive voltage offsets", "Force writing positive voltage offsets (useful for overclocking). Be very careful. This is danger zone and may permanantly damage your CPU or other components if you don't 100% know what you are doing.", showWarning)

	lockCheck := newInfoCheck("Lock power limit", "Lock the power limit settings to prevent changes.", showWarning)

	verboseCheck := newInfoCheck("Enable Verbose Output", "Enable detailed output from undervolt-go.", showWarning)

	turboOptions := map[string]string{
		"Default":  "-1",
		"Enabled":  "0",
		"Disabled": "1",
	}
	turboSelect := newInfoSelect([]string{"Default", "Enabled", "Disabled"}, "Control turbo mode: Default (No change), Enabled (Allow turbo), Disabled (Block turbo).", showWarning)

	// Temperature Inputs
	tempEntry := newInfoEntry("Maximum temperature on AC power (°C).", showWarning)
	tempEntry.SetPlaceHolder("AC °C")

	tempBatEntry := newInfoEntry("Maximum temperature on battery (°C).", showWarning)
	tempBatEntry.SetPlaceHolder("Battery °C")
	for _, e := range []*infoEntry{tempEntry, tempBatEntry} {
		e.Validator = intValidator
	}

	// Settings Inputs
	persistCheck := widget.NewCheck("Persist the undervolt configuration on reboots", nil)
	persistInfo := widget.NewLabel("Make sure the configuration being persisted is indeed a stable one. Untick this checkbox when not needed.")
	persistInfo.Wrapping = fyne.TextWrapWord

	// Profile Selects
	profileSaveSelect := widget.NewSelect([]string{"AC", "Battery"}, nil)
	profileLoadSelect := widget.NewSelect([]string{"Auto", "AC", "Battery"}, nil)

	// Flag Collection & Runner
	collect := func() []string {
		var args []string
		var outputLabelAlertArray []string
		for _, p := range planes {
			if p.check.Checked {
				args = append(args, "--"+p.command+"="+p.entry.Text)
			} else {
				outputLabelAlertArray = append(outputLabelAlertArray, "Voltage offset for "+p.name+" was not applied as the corresponding checkbox is unchecked.\n")
			}
		}
		if len(outputLabelAlertArray) > 0 {
			outputLabelBinding.Set(strings.Join(outputLabelAlertArray, "\n"))
			showWarning("Error occured when applying Voltage Offset settings. Please check 'Output' pane for more information.", 3*time.Second)
		}

		if forceCheck.Checked {
			args = append(args, "--force")
		}
		if lockCheck.Checked {
			args = append(args, "--lock-power-limit")
		}
		if verboseCheck.Checked {
			args = append(args, "--verbose")
		}
		if val, ok := turboOptions[turboSelect.Selected]; ok {
			args = append(args, "--turbo="+val)
		}
		if tempEntry.Text != "" {
			args = append(args, "--temp="+tempEntry.Text)
		}
		if tempBatEntry.Text != "" {
			args = append(args, "--temp-bat="+tempBatEntry.Text)
		}
		if p1Power.Text != "" && p1Time.Text != "" {
			args = append(args, "--p1="+p1Power.Text+","+p1Time.Text)
		}
		if p2Power.Text != "" && p2Time.Text != "" {
			args = append(args, "--p2="+p2Power.Text+","+p2Time.Text)
		}
		if persistCheck.Checked {
			args = append(args, "--persist")
		}
		return args
	}

	//
	run := func(flags ...string) error {
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
			showWarning("Error occured when applying settings. Please check 'Output' pane for more information.", 3*time.Second)
		}
		outputLabelBinding.Set(buf.String())
		return err
	}

	// Sections & Their Scrollable Content
	secNames := []string{"Voltage Offset", "Power Limit", "Temperature Limits", "Other Flags", "Profiles", "Settings", "Status"}
	sectionHeader := make(map[string]*widget.Label)
	// Create section headers
	for _, s := range secNames {
		sectionHeader[s] = widget.NewLabelWithStyle(
			s,
			fyne.TextAlignLeading,
			fyne.TextStyle{Bold: true},
		)
	}
	// secContainers is a map of section names to their content containers.
	// This map is used to store the content containers for each section,
	// so that we can easily access them later.
	secContainers := make(map[string]fyne.CanvasObject)

	// Voltage Offset section
	voltOffsetSection := container.NewVBox()
	voltOffsetSection.Add(sectionHeader["Voltage Offset"])
	voltOffsetCont := container.New(layout.NewFormLayout())
	for _, p := range planes {
		voltOffsetCont.Add(container.New(layout.NewHBoxLayout(), p.check, widget.NewLabel(p.name)))
		voltOffsetCont.Add(p.entry)
	}
	voltOffsetSection.Add(voltOffsetCont)
	secContainers["Voltage Offset"] = container.NewVScroll(voltOffsetSection)

	// Power Limit section
	plSection := container.NewVBox()
	plSection.Add(sectionHeader["Power Limit"])
	plForm := container.New(layout.NewFormLayout(),
		widget.NewLabel("P1 Power (W)"), p1Power,
		widget.NewLabel("P1 Time (s)"), p1Time,
		widget.NewLabel("P2 Power (W)"), p2Power,
		widget.NewLabel("P2 Time (s)"), p2Time,
	)
	plSection.Add(plForm)
	secContainers["Power Limit"] = container.NewVScroll(plSection)

	// Temperature Limits section
	tempSection := container.NewVBox()
	tempSection.Add(sectionHeader["Temperature Limits"])
	tempGrid := container.New(layout.NewFormLayout(),
		widget.NewLabel("AC Temp (°C)"), tempEntry,
		widget.NewLabel("Battery Temp (°C)"), tempBatEntry,
	)
	tempSection.Add(tempGrid)
	secContainers["Temperature Limits"] = container.NewVScroll(tempSection)

	// Other Flags section
	otherFlagsSection := container.NewVBox(sectionHeader["Other Flags"], forceCheck, lockCheck, verboseCheck, container.NewHBox(), container.New(layout.NewFormLayout(), widget.NewLabel("Turbo"), turboSelect))
	secContainers["Other Flags"] = container.NewVScroll(otherFlagsSection)

	// Profiles Section
	profileSaveBtn := widget.NewButton("Save Profile", func() {
		name := profileSaveSelect.Selected
		if name == "" {
			showWarning("Please select a profile to save.", 3*time.Second)
			return
		}
		args := collect()
		flags := append([]string{"profile", "save", strings.ToLower(name)}, args...)
		if len(args) > 0 {
			if err := run(flags...); err == nil {
				showWarning("Settings saved successfully as profile "+name+".", 3*time.Second)
			}
		}
	})

	profileLoadBtn := widget.NewButton("Load Profile", func() {
		name := profileLoadSelect.Selected
		if name == "" {
			showWarning("Please select a profile to load.", 3*time.Second)
			return
		}

		// Kind of a duplication of code from main.go
		actualName := name
		if actualName == "Auto" {
			if isBatteryDischarging() {
				actualName = "Battery"
			} else {
				actualName = "AC"
			}
		}

		// Reload config from disk to catch updated profiles
		initConfig()

		key := "profiles." + strings.ToLower(actualName)
		if !viper.IsSet(key) {
			showWarning(fmt.Sprintf("Profile '%s' not found.", actualName), 3*time.Second)
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
		// we can use p.GetIntSlice here as the values are int. we couldn't in main.go as the flags were string.
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
		for _, p := range planes {
			switch p.name {
			case "Core":
				p.entry.SetText(fmt.Sprintf("%f", coreOffset))
			case "Cache":
				p.entry.SetText(fmt.Sprintf("%f", cacheOffset))
			case "GPU":
				p.entry.SetText(fmt.Sprintf("%f", gpuOffset))
			case "Uncore":
				p.entry.SetText(fmt.Sprintf("%f", uncoreOffset))
			case "AnalogIO":
				p.entry.SetText(fmt.Sprintf("%f", analogioOffset))
			}
		}
		if len(p1Args) == 2 {
			p1Power.SetText(fmt.Sprintf("%d", p1Args[0]))
			p1Time.SetText(fmt.Sprintf("%d", p1Args[1]))
		} else {
			p1Power.SetText("")
			p1Time.SetText("")
		}
		if len(p2Args) == 2 {
			p2Power.SetText(fmt.Sprintf("%d", p2Args[0]))
			p2Time.SetText(fmt.Sprintf("%d", p2Args[1]))
		} else {
			p2Power.SetText("")
			p2Time.SetText("")
		}

		tempEntry.SetText(fmt.Sprintf("%d", tempFlag))
		tempBatEntry.SetText(fmt.Sprintf("%d", tempBatFlag))

		turboProfile := ""
		for option, value := range turboOptions {
			if value == strconv.Itoa(turboFlag) {
				turboProfile = option
				break
			}
		}
		if turboProfile != "" {
			turboSelect.SetSelected(turboProfile)
		} else {
			turboSelect.ClearSelected()
		}

		showWarning(fmt.Sprintf("Profile '%s' loaded into the UI.", actualName), 3*time.Second)
	})

	// Helper to check if the auto-switch udev rule exists
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
	updateAutoSwitchBtn() // Set initial text

	autoSwitchBtn.OnTapped = func() {
		initConfig() // Make sure we have the latest config state

		if !isAutoSwitchEnabled() {
			// Check if profiles exist before allowing it to be enabled
			if !viper.IsSet("profiles.ac") || !viper.IsSet("profiles.battery") {
				showWarning("Both 'AC' and 'Battery' profiles must exist before enabling auto-profile switching.", 4*time.Second)
				return
			}

			// Execute backend command
			if err := run("profile", "auto-switch", "enable"); err == nil {
				showWarning("Auto profile switching enabled.", 3*time.Second)
			}
		} else {
			if err := run("profile", "auto-switch", "disable"); err == nil {
				showWarning("Auto profile switching disabled.", 3*time.Second)
			}
		}
		updateAutoSwitchBtn() // Update text after clicking
	}

	profilesSection := container.NewVBox(
		sectionHeader["Profiles"],
		widget.NewLabel("Save current settings to a profile:"),
		profileSaveSelect,
		profileSaveBtn,
		widget.NewLabel(""),
		widget.NewLabel("Load settings from a profile:"),
		profileLoadSelect,
		profileLoadBtn,
		widget.NewLabel(""),
		widget.NewSeparator(),
		autoSwitchLabel,
		autoSwitchBtn,
	)
	secContainers["Profiles"] = container.NewVScroll(profilesSection)

	// Settings section
	clearPersistBtn := widget.NewButton("Clear persisted configuration", func() {
		if err := run("--disable-persist"); err == nil {
			showWarning("Persisted configuration cleared successfully.", 3*time.Second)
		}
	})

	settingsSection := container.NewVBox(
		sectionHeader["Settings"],
		persistCheck,
		persistInfo,
		widget.NewLabel(""),
		clearPersistBtn,
	)
	secContainers["Settings"] = container.NewVScroll(settingsSection)

	// Output section
	// Monitoring state
	// these let us start/stop a 1 Hz loop
	var (
		monitorTicker *time.Ticker
		stopMonitor   chan struct{}
	)

	// startMonitor spins up a goroutine that every second runs `command`
	// and dumps its stdout/stderr into outputLabel (including any errors).
	startMonitor := func(command string) {
		if monitorTicker != nil {
			return // already running
		}
		stopMonitor = make(chan struct{})
		monitorTicker = time.NewTicker(1 * time.Second)
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
					text := buf.String()
					// update UI on main thread safely
					outputLabelBinding.Set(text)
				}
			}
		}(stopMonitor, monitorTicker)
	}

	// stopMonitorFunc tells that goroutine to exit
	stopMonitorFunc := func() {
		if stopMonitor != nil {
			close(stopMonitor) // Wakes up the blocking select statement
			stopMonitor = nil
		}
		// Safely clear the ticker state purely on the main UI thread to avoid data races
		if monitorTicker != nil {
			monitorTicker.Stop()
			monitorTicker = nil
		}
	}

	// Buttons
	readBtn := widget.NewButton("Read", func() {
		if monitorTicker != nil {
			showWarning("Please click 'Stop' before running this command.", 3*time.Second)
			return
		}
		_ = run("--read")
	})
	helpBtn := widget.NewButton("Help", func() {
		if monitorTicker != nil {
			showWarning("Please click 'Stop' before running this command.", 3*time.Second)
			return
		}
		_ = run("--help")
	})
	checkTempsBtn := widget.NewButton("Check Temps", func() {
		if monitorTicker != nil {
			showWarning("Please click 'Stop' before running this command.", 3*time.Second)
			return
		}
		showWarning("Please click 'Stop' before running any other command or closing the app.", 3*time.Second)
		startMonitor("sensors | grep Core")
	})
	checkFansBtn := widget.NewButton("Check Fans", func() {
		if monitorTicker != nil {
			showWarning("Please click 'Stop' before running this command.", 3*time.Second)
			return
		}
		showWarning("Please click 'Stop' before running any other command or closing the app.", 3*time.Second)
		startMonitor("sensors | grep -e cpu_fan -e gpu_fan")
	})
	stopBtn := widget.NewButton("Stop", func() {
		if monitorTicker == nil {
			showWarning("Nothing to stop there. You're good. :)", 3*time.Second)
			return
		}
		stopMonitorFunc()
		showWarning("Monitoring stopped. You may now run other commands.", 3*time.Second)
	})
	verBtn := widget.NewButton("Version", func() {
		if monitorTicker != nil {
			showWarning("Please click 'Stop' before running this command.", 3*time.Second)
			return
		}
		_ = run("--version")
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

	// OUTPUT SECTION
	outputSection := container.NewBorder(
		// Top: the Output header
		sectionHeader["Status"],
		// Bottom: warning + buttons
		btnBar,
		// Left / Right: none
		nil, nil,
		// Center: let the VScroll fill all remaining space
		container.NewVScroll(outputLabel),
	)

	secContainers["Status"] = outputSection

	// Section container (only one visible at a time)
	sectionContainer := container.NewMax()
	showSection := func(name string) {
		sectionContainer.Objects = []fyne.CanvasObject{secContainers[name]}
		sectionContainer.Refresh()
	}
	showSection(secNames[0]) // start on the first
	sectionContent := container.NewBorder(
		nil, outputWarning, nil, nil, sectionContainer,
	)

	// Sidebar (33% width)
	sideBtns := make([]fyne.CanvasObject, len(secNames))
	for i, name := range secNames {
		n := name
		sideBtns[i] = widget.NewButton(n, func() { showSection(n) })
	}
	// Link to Website
	siteURL, _ := url.Parse("https://softorage.com")
	authorLink := widget.NewHyperlink("Softorage", siteURL)
	authorLink.Alignment = fyne.TextAlignCenter
	// the content for sidebar is consolidated in sidebarContent
	sidebarContent := container.NewVBox(
		append(sideBtns,
			layout.NewSpacer(), // pushes link to bottom
			container.NewHBox(layout.NewSpacer(), widget.NewLabel("by"), authorLink),
		)...,
	)
	// the sidebar
	sidebar := container.NewVScroll(sidebarContent)

	// Action Buttons (docked bottom‑right)
	settingsApplyBtn := widget.NewButton("Apply", func() {
		// without len(collect()) > 0, clicking on apply without any setting relaunches another window of Undervolt Go
		if len(collect()) > 0 {
			if err := run(collect()...); err == nil {
				showWarning("Settings applied successfully.", 3*time.Second)
			}
		}
	})

	mainBtnBar := container.NewHBox(layout.NewSpacer(), settingsApplyBtn)

	// Combine: HSplit + Border for button bar
	split := container.NewHSplit(sidebar, sectionContent)
	split.SetOffset(0.33) // sidebar gets 33% of width
	content := container.NewBorder(nil, mainBtnBar, nil, nil, split)

	w.SetContent(content)
	w.ShowAndRun()
}