//go:build gui

// gui.go

/*
✅ 5. Build GUI version

go build -tags gui -o undervolt-go-gui .
go build -tags gui -ldflags="-X main.version=$(git describe --tags)" -o undervolt-go-pro .
✅ This will include gui.go, and runGUI() will launch your Fyne GUI.
*/

package main

import (
	"bytes"
	"fmt"
	"os/exec"

	//"os" remove comment and import os if code for allowing notification for non-sudo user is uncommented
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

  // --- Output Pane (read-only) ---
  outputWarningBind := binding.NewString()
  outputWarning := widget.NewLabelWithData(outputWarningBind)

  // --- Voltage Offset Inputs (with enable checkboxes) ---
  type planeUI struct {
    name    string
    command string
    entry   *widget.Entry
    check   *widget.Check
  }
  planes := []planeUI{
    {"Core", "core", widget.NewEntry(), widget.NewCheck("", nil)},
    {"Cache", "cache", widget.NewEntry(), widget.NewCheck("", nil)},
    {"GPU", "gpu", widget.NewEntry(), widget.NewCheck("", nil)},
    {"Uncore", "uncore", widget.NewEntry(), widget.NewCheck("", nil)},
    {"AnalogIO", "analogio", widget.NewEntry(), widget.NewCheck("", nil)},
  }
  for _, p := range planes {
    p.entry.SetPlaceHolder("e.g. -50.000")
    //p.entry.SetTooltip("Voltage offset for " + p.name + " plane (e.g., -50.000 mV)")
    //p.check.SetTooltip("Enable undervolt for " + p.name + " plane")

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

  // --- Power Limit Inputs ---
  p1Power := widget.NewEntry()
  p1Power.SetPlaceHolder("Power (W)")
  //p1Power.SetTooltip("P1 power limit in watts (e.g., 45) is the long term power limit, that can be safe for longer periods")

  p1Time := widget.NewEntry()
  p1Time.SetPlaceHolder("Time (s)")
  //p1Time.SetTooltip("Time window for P1 in seconds (e.g., 28)")

  p2Power := widget.NewEntry()
  p2Power.SetPlaceHolder("Power (W)")
  //p2Power.SetTooltip("P2 power limit in watts (e.g., 60) is the short term power limit, that can be safe for shorter periods and is useful for short bursts of performance.")

  p2Time := widget.NewEntry()
  p2Time.SetPlaceHolder("Time (s)")
  //p2Time.SetTooltip("Time window for P2 in seconds (e.g., 2)")

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
  for _, e := range []*widget.Entry{p1Power, p1Time, p2Power, p2Time} {
    e.Validator = intValidator
  }

  // --- Other Flags ---
  forceCheck := widget.NewCheck("Force positive voltage offsets", nil)
  //forceCheck.SetTooltip("Force writing positive voltage offsets (useful for overclocking). Be very careful. This is danger zone and may permanantly damage your CPU or other components if you don't 100% know what you are doing.")

  lockCheck := widget.NewCheck("Lock power limit", nil)
  //lockCheck.SetTooltip("Lock the power limit settings to prevent changes")

  verboseCheck := widget.NewCheck("Enable Verbose Output", nil)
  //verboseCheck.SetTooltip("Enable detailed output from undervolt-go")

  turboOptions := map[string]string{
    "Default":  "-1",
    "Enabled":  "0",
    "Disabled": "1",
  }
  turboSelect := widget.NewSelect([]string{"Default", "Enabled", "Disabled"}, nil)
  //turboSelect.SetTooltip("Control turbo mode:\n- Default: No change\n- Enabled: Allow turbo\n- Disabled: Block turbo")
  //SetTooltip(turboSelect, "Control turbo mode:\n- Default: No change\n- Enabled: Allow turbo\n- Disabled: Block turbo")

  // --- Temperature Inputs ---
  tempEntry := widget.NewEntry()
  tempEntry.SetPlaceHolder("AC °C")
  //tempEntry.SetTooltip("Maximum temperature on AC power (°C)")

  tempBatEntry := widget.NewEntry()
  tempBatEntry.SetPlaceHolder("Battery °C")
  //tempBatEntry.SetTooltip("Maximum temperature on battery (°C)")
  for _, e := range []*widget.Entry{tempEntry, tempBatEntry} {
    e.Validator = intValidator
  }

  // --- Flag Collection & Runner ---
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
      outputWarningBind.Set("Error occured when applying Voltage Offset settings. Please check 'Output' pane for more information.")
      clearLabelAfter(outputWarningBind, 3*time.Second)
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
    return args
  }

  //
  run := func(flags ...string) error {
    cmd := exec.Command("sudo", append([]string{"undervolt-go-pro"}, flags...)...)
    // Redirect command output to a buffer for display in the Output Pane.
    // Redirect both stdout and stderr to the same buffer, so that any error
    // messages are included in the output.
    var buf bytes.Buffer
    cmd.Stdout = &buf
    cmd.Stderr = &buf
    err := cmd.Run()
    if err != nil {
      buf.WriteString("\nError: " + err.Error())
      outputWarningBind.Set("Error occured when applying settings. Please check 'Output' pane for more information.")
      clearLabelAfter(outputWarningBind, 3*time.Second)
    }
    outputLabelBinding.Set(buf.String())
    return err
  }

  // --- Sections & Their Scrollable Content ---
  secNames := []string{"Voltage Offset", "Power Limit", "Temperature Limits", "Other Flags", "Output"}
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

  // Output section
  // ─── Monitoring state ───────────────────────────────────────────────────────────
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
    go func() {
      for {
        select {
        case <-stopMonitor:
          monitorTicker.Stop()
          monitorTicker = nil
          return
        case <-monitorTicker.C:
          // run via shell so pipes/greps work
          cmd := exec.Command("sh", "-c", command)
          var buf bytes.Buffer
          cmd.Stdout = &buf
          cmd.Stderr = &buf
          if err := cmd.Run(); err != nil {
            buf.WriteString("\nError: " + err.Error())
          }
          text := buf.String()
          // update UI on main thread
          outputLabelBinding.Set(text)
        }
      }
    }()
  }

  // stopMonitorFunc tells that goroutine to exit
  stopMonitorFunc := func() {
    if stopMonitor != nil {
      close(stopMonitor)
      stopMonitor = nil
    }
  }

  // ─── Buttons ───────────────────────────────────────────────────────────────────
  readBtn := widget.NewButton("Read", func() {
    if monitorTicker != nil {
      outputWarningBind.Set("Please click 'Stop' before running this command.")
      clearLabelAfter(outputWarningBind, 3*time.Second)
      return
    }
    _ = run("--read")
  })
  helpBtn := widget.NewButton("Help", func() {
    if monitorTicker != nil {
      outputWarningBind.Set("Please click 'Stop' before running this command.")
      clearLabelAfter(outputWarningBind, 3*time.Second)
      return
    }
    _ = run("--help")
  })
  checkTempsBtn := widget.NewButton("Check Temps", func() {
    if monitorTicker != nil {
      outputWarningBind.Set("Please click 'Stop' before running this command.")
      clearLabelAfter(outputWarningBind, 3*time.Second)
      return
    }
    outputWarningBind.Set("Please click 'Stop' before running any other command or closing the app.")
    clearLabelAfter(outputWarningBind, 3*time.Second)
    startMonitor("sensors | grep Core")
  })
  checkFansBtn := widget.NewButton("Check Fans", func() {
    if monitorTicker != nil {
      outputWarningBind.Set("Please click 'Stop' before running this command.")
      clearLabelAfter(outputWarningBind, 3*time.Second)
      return
    }
    outputWarningBind.Set("Please click 'Stop' before running any other command or closing the app.")
    clearLabelAfter(outputWarningBind, 3*time.Second)
    startMonitor("sensors | grep -e cpu_fan -e gpu_fan")
  })
  stopBtn := widget.NewButton("Stop", func() {
    if monitorTicker == nil {
      outputWarningBind.Set("Nothing to stop there. You're good. :)")
      clearLabelAfter(outputWarningBind, 3*time.Second)
      return
    }
    stopMonitorFunc()
    outputWarningBind.Set("Monitoring stopped. You may now run other commands.")
    clearLabelAfter(outputWarningBind, 3*time.Second)
  })
  verBtn := widget.NewButton("Version", func() {
    if monitorTicker != nil {
      outputWarningBind.Set("Please click 'Stop' before running this command.")
      clearLabelAfter(outputWarningBind, 3*time.Second)
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

  // ─── OUTPUT SECTION ────────────────────────────────────────────────────────────
  outputSection := container.NewBorder(
    // Top: the “Output” header
    sectionHeader["Output"],
    // Bottom: warning + buttons
    btnBar,
    // Left / Right: none
    nil, nil,
    // Center: let the VScroll fill all remaining space
    container.NewVScroll(outputLabel),
  )

  secContainers["Output"] = outputSection

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

  // --- Sidebar (33% width) ---
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

  // --- Action Buttons (docked bottom‑right) ---
  settingsApplyBtn := widget.NewButton("Apply", func() {
    // without len(collect()) > 0, clicking on apply without any setting relaunches another window of Undervolt Go
    if len(collect()) > 0 {
      if err := run(collect()...); err == nil {
        outputWarningBind.Set("Settings applied successfully.")
        clearLabelAfter(outputWarningBind, 3*time.Second)
      }
    }
  })

  profileSelect := widget.NewSelect([]string{"AC", "Battery"}, nil)

  profileSaveBtn := widget.NewButton("Save Profile", func() {
    name := profileSelect.Selected
    args := collect()
    fmt.Println(args)
    fmt.Println(name)
    flags := append([]string{"profile", "save", strings.ToLower(name)}, args...)
    if len(args) > 0 {
      if err := run(flags...); err == nil {
        outputWarningBind.Set("Settings saved successfully as profile " + name + ".")
        clearLabelAfter(outputWarningBind, 3*time.Second)
      }
    }
  })

  profileLoadBtn := widget.NewButton("Load Profile", func() {
    name := profileSelect.Selected
    //updateViaProfile(name)
    key := "profiles." + name
    if !viper.IsSet(key) {
      fmt.Printf("Profile '%s' not found.\n", name)
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

    p1Args := p.GetIntSlice("pl.p1")
    p2Args := p.GetIntSlice("pl.p2")
    fmt.Println(p1Args, p2Args)
    /* below code is useful if the values of p1 or p2 array float
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
    }
    if len(p2Args) == 2 {
      p2Power.SetText(fmt.Sprintf("%d", p2Args[0]))
      p2Time.SetText(fmt.Sprintf("%d", p2Args[1]))
    }
    // Update the values in the check widgets
    // Assuming you have check widgets for temp, temp-bat, and turbo
    tempEntry.SetText(fmt.Sprintf("%d", tempFlag))
    tempBatEntry.SetText(fmt.Sprintf("%d", tempBatFlag))

    turboProfile := ""
    for option, value := range turboOptions {
      if value == strconv.Itoa(turboFlag) {
        turboProfile = option
        break
      }
    }
    turboSelect.SetSelected(turboProfile)
  })

  mainBtnBar := container.NewHBox(layout.NewSpacer(), profileSaveBtn, profileLoadBtn, settingsApplyBtn)
  mainMenuBar := container.NewHBox(layout.NewSpacer(), widget.NewLabel("Profile: "), profileSelect)

  // --- Combine: HSplit + Border for button bar ---
  split := container.NewHSplit(sidebar, sectionContent)
  split.SetOffset(0.33) // sidebar gets 33% of width
  content := container.NewBorder(mainMenuBar, mainBtnBar, nil, nil, split)

  w.SetContent(content)
  w.ShowAndRun()
}

// Helper function
// a function to clear label after specified time in seconds
func clearLabelAfter(binding binding.String, d time.Duration) {
  go func() {
    time.Sleep(d)
    _ = binding.Set("")
  }()
}
