// main.go
// Go port of undervolt.py (v0.4.0) including power limit adjustments using cobra.
// WARNING: Undervolting and power limit changes are dangerous; use at your own risk.

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Version
var version = "dev"

// Mapping of voltage planes to indices.
var planes = map[string]int{
	"core":     0,
	"gpu":      1,
	"cache":    2,
	"uncore":   3,
	"analogio": 4,
	// "digitalio": 5, // not working
}

// MSR holds addresses of registers.
type MSR struct {
	addrVoltageOffsets uint64
	addrUnits          uint64
	addrPowerLimits    uint64
	addrTemp           uint64
}

// Default addresses (for Core iX 6th–9th gen etc.)
var ADDRESSES = MSR{
	addrVoltageOffsets: 0x150,
	addrUnits:          0x606,
	addrPowerLimits:    0x610,
	addrTemp:           0x1a2,
}

// PowerLimit holds the power limit settings.
type PowerLimit struct {
	ShortTermEnabled bool
	ShortTermPower   float64 // in Watts
	ShortTermTime    float64 // in seconds
	LongTermEnabled  bool
	LongTermPower    float64 // in Watts
	LongTermTime     float64 // in seconds
	Locked           bool
	BackupRest       uint64
}

// Pre-allocate byte slices for sysfs zero-allocation parsing
var (
	batteryType   = []byte("Battery")
	dischargingSt = []byte("Discharging")
)

// isBatteryDischarging returns true if *any* battery in /sys/class/power_supply is discharging.
// Optimized to operate directly on byte slices to avoid heap string allocations.
func isBatteryDischarging() bool {
	base := "/sys/class/power_supply"
	entries, err := os.ReadDir(base)
	if err != nil {
		return false
	}
	for _, e := range entries {
		dir := filepath.Join(base, e.Name())

		t, err := os.ReadFile(filepath.Join(dir, "type"))
		if err != nil || !bytes.HasPrefix(t, batteryType) {
			continue
		}

		st, err := os.ReadFile(filepath.Join(dir, "status"))
		if err != nil {
			continue
		}
		// “Discharging” indicates battery is being drained
		if bytes.HasPrefix(st, dischargingSt) {
			return true
		}
	}
	return false
}

// ---------- MSR Read/Write Functions ----------

// validCPUs returns CPU indices with an available /dev/cpu/<i> directory.
func validCPUs() ([]int, error) {
	var cpus []int
	n := runtime.NumCPU()
	for i := 0; i < n; i++ {
		path := fmt.Sprintf("/dev/cpu/%d", i)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			cpus = append(cpus, i)
		}
	}
	return cpus, nil
}

// writeMSR writes an 8-byte little-endian value to the given address on all CPUs concurrently.
func writeMSR(val uint64, addr uint64) error {
	cpus, err := validCPUs()
	if err != nil {
		return err
	}

	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, val)

	var wg sync.WaitGroup
	errCh := make(chan error, len(cpus))

	// Write to all CPU MSRs concurrently to minimize voltage state skewness
	for _, cpu := range cpus {
		wg.Add(1)
		go func(cpu int) {
			defer wg.Done()
			path := fmt.Sprintf("/dev/cpu/%d/msr", cpu)
			f, err := os.OpenFile(path, os.O_WRONLY, 0)
			if err != nil {
				if os.IsPermission(err) {
					errCh <- fmt.Errorf("permission denied to %s (is Secure Boot / Kernel Lockdown enabled?)", path)
					return
				}
				errCh <- err
				return
			}

			// Use WriteAt to map to the 'pwrite' syscall directly, avoiding an extra 'lseek' syscall
			_, err = f.WriteAt(buf, int64(addr))
			f.Close() // Close immediately

			if err != nil {
				errCh <- err
			} else {
				log.Printf("Successfully wrote 0x%x to %s", val, path)
			}
		}(cpu)
	}

	wg.Wait()
	close(errCh)

	// Return the first error if any
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// readMSR reads an 8-byte little-endian value from the given address on the specified CPU.
func readMSR(addr uint64, cpu int) (uint64, error) {
	path := fmt.Sprintf("/dev/cpu/%d/msr", cpu)
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		if os.IsPermission(err) {
			return 0, fmt.Errorf("permission denied to %s (is Secure Boot / Kernel Lockdown enabled?)", path)
		}
		return 0, err
	}
	defer f.Close()

	buf := make([]byte, 8)
	// Use ReadAt to map to the 'pread' syscall directly, avoiding an extra 'lseek' syscall
	if _, err := f.ReadAt(buf, int64(addr)); err != nil {
		return 0, err
	}
	val := binary.LittleEndian.Uint64(buf)
	log.Printf("Read 0x%x from %s", val, path)
	return val, nil
}

// ---------- Voltage Offset Functions ----------

// convertRoundedOffset computes: 0xFFE00000 & ((x & 0xFFF) << 21)
func convertRoundedOffset(x int) uint32 {
	return uint32(0xFFE00000) & (uint32(x&0xFFF) << 21)
}

// convertOffset converts an mV offset to an MSR-compatible offset.
func convertOffset(mV float64) uint32 {
	rounded := int(math.Round(mV * 1.024))
	return convertRoundedOffset(rounded)
}

// unconvertRoundedOffset reverses convertRoundedOffset.
func unconvertRoundedOffset(y uint32) int {
	x := y >> 21
	if x <= 1024 {
		return int(x)
	}
	return -(2048 - int(x))
}

// unconvertOffset returns the offset in mV.
func unconvertOffset(y uint32) float64 {
	return float64(unconvertRoundedOffset(y)) / 1.024
}

// packOffset constructs an MSR value to read or write an offset for a plane.
func packOffset(planeIndex int, offset uint32, write bool) uint64 {
	var hasOffset uint64 = 0
	if write {
		hasOffset = 1
	}
	// ((1 << 63) | (planeIndex << 40) | (1 << 36) | (hasOffset << 32) | off)
	return (1 << 63) | (uint64(planeIndex) << 40) | (1 << 36) | (hasOffset << 32) | uint64(offset)
}

// unpackOffset extracts the voltage offset (in mV) from an MSR response.
func unpackOffset(msrResponse uint64) float64 {
	planeIndex := int(msrResponse >> 40)
	value := uint32(msrResponse ^ (uint64(planeIndex) << 40))
	return unconvertOffset(value)
}

// readTemperature extracts the temperature target.
func readTemperature(msr MSR) (int, error) {
	val, err := readMSR(msr.addrTemp, 0)
	if err != nil {
		return 0, err
	}
	temp := int((val & (127 << 24)) >> 24)
	return temp, nil
}

// setTemperature sets a new temperature target (in °C).
func setTemperature(temp int, msr MSR) error {
	value := uint64((100 - temp) << 24)
	return writeMSR(value, msr.addrTemp)
}

// readOffset sends a "read" command for the voltage offset and returns the measured value.
func readOffset(plane string, msr MSR) (float64, error) {
	planeIndex, ok := planes[plane]
	if !ok {
		return 0, fmt.Errorf("unknown plane: %s", plane)
	}
	valueToWrite := packOffset(planeIndex, 0, false)
	if err := writeMSR(valueToWrite, msr.addrVoltageOffsets); err != nil {
		return 0, err
	}
	val, err := readMSR(msr.addrVoltageOffsets, 0)
	if err != nil {
		return 0, err
	}
	return unpackOffset(val), nil
}

// setOffset applies a new voltage offset (in mV) to a given plane.
func setOffset(plane string, mV float64, msr MSR, force bool) error {
	planeIndex, ok := planes[plane]
	if !ok {
		return fmt.Errorf("unknown plane: %s", plane)
	}
	if mV > 0 && !force {
		return fmt.Errorf("positive offset requires --force")
	}
	log.Printf("Setting %s offset to %.2f mV", plane, mV)
	target := convertOffset(mV)
	writeValue := packOffset(planeIndex, target, true)
	if err := writeMSR(writeValue, msr.addrVoltageOffsets); err != nil {
		return err
	}
	// Verify that the value was applied.
	wantMV := unconvertOffset(target)
	readMV, err := readOffset(plane, msr)
	if err != nil {
		return err
	}
	if math.Abs(wantMV-readMV) > 0.001 {
		return fmt.Errorf("failed to apply %s: set %.2f, read %.2f", plane, wantMV, readMV)
	}
	return nil
}

// ---------- Power Limit Functions ----------

func toSeconds(val uint64, unit float64) float64 {
	return math.Pow(2, float64(val&0x1f)) * (1 + float64((val>>5)&0x3)/4.0) / unit
}

func readPowerLimit(msr MSR) (PowerLimit, error) {
	var pl PowerLimit
	units, err := readMSR(msr.addrUnits, 0)
	if err != nil {
		return pl, err
	}
	val, err := readMSR(msr.addrPowerLimits, 0)
	if err != nil {
		return pl, err
	}
	powerUnit := math.Pow(2, float64(units&0xf))
	timeUnit := math.Pow(2, float64((units>>16)&0xf))
	pl.ShortTermEnabled = ((val >> 47) & 0x1) != 0
	pl.ShortTermPower = float64((val>>32)&0x7fff) / powerUnit
	pl.ShortTermTime = toSeconds(val>>49, timeUnit)
	pl.LongTermEnabled = ((val >> 15) & 0x1) != 0
	pl.LongTermPower = float64(val&0x7fff) / powerUnit
	pl.LongTermTime = toSeconds(val>>17, timeUnit)
	pl.Locked = ((val >> 63) & 1) != 0
	pl.BackupRest = val & 0x7f010000ff010000
	return pl, nil
}

func fromSeconds(val float64, unit float64) uint64 {
	product := val * unit
	if math.Log2(product/1.75) >= 31 { // 0x1f = 31
		return 0xfe
	}
	minErr := 1e9
	var result uint64 = 0
	for y := 0; y < 4; y++ {
		multiplier := 1 + float64(y)/4.0
		valMult := product / multiplier
		exp := math.Log2(valMult)
		expInt := int(exp)
		if (valMult - math.Pow(2, float64(expInt))) >= (math.Pow(2, float64(expInt+1)) - valMult) {
			expInt++
		}
		if expInt > 31 {
			expInt = 31
		}
		backVal := math.Pow(2, float64(expInt)) * multiplier
		errVal := math.Abs(backVal - product)
		if errVal < minErr {
			minErr = errVal
			result = (uint64(y) << 5) | uint64(expInt)
		}
	}
	return result
}

func setPowerLimit(pl PowerLimit, msr MSR) error {
	oldPl, err := readPowerLimit(msr)
	if err != nil {
		return err
	}
	if oldPl.Locked {
		return fmt.Errorf("cannot write power limit because it is locked")
	}
	units, err := readMSR(msr.addrUnits, 0)
	if err != nil {
		return err
	}
	powerUnit := math.Pow(2, float64(units&0xf))
	timeUnit := math.Pow(2, float64((units>>16)&0xf))
	writeValue := oldPl.BackupRest

	// Short term settings.
	stEnabled := oldPl.ShortTermEnabled
	stPower := oldPl.ShortTermPower
	stTime := oldPl.ShortTermTime
	if pl.ShortTermPower > 0 {
		stEnabled = true
		stPower = pl.ShortTermPower
		stTime = pl.ShortTermTime
	}
	if stEnabled {
		writeValue |= (1 << 47)
	}
	stPowerVal := int(stPower * powerUnit)
	if stPowerVal < 0 || stPowerVal > 0x7fff {
		return fmt.Errorf("short term power out of range (%d > 0x7fff)", stPowerVal)
	}
	writeValue |= uint64(stPowerVal) << 32
	stTimeVal := fromSeconds(stTime, timeUnit)
	writeValue |= stTimeVal << 49

	// Long term settings.
	ltEnabled := oldPl.LongTermEnabled
	ltPower := oldPl.LongTermPower
	ltTime := oldPl.LongTermTime
	if pl.LongTermPower > 0 {
		ltEnabled = true
		ltPower = pl.LongTermPower
		ltTime = pl.LongTermTime
	}
	if ltEnabled {
		writeValue |= (1 << 15)
	}
	ltPowerVal := int(ltPower * powerUnit)
	if ltPowerVal < 0 || ltPowerVal > 0x7fff {
		return fmt.Errorf("long term power out of range (%d > 0x7fff)", ltPowerVal)
	}
	writeValue |= uint64(ltPowerVal)
	ltTimeVal := fromSeconds(ltTime, timeUnit)
	writeValue |= ltTimeVal << 17

	// Locked flag.
	locked := oldPl.Locked
	if pl.Locked {
		locked = true
	}
	if locked {
		writeValue |= (1 << 63)
	}

	if err := writeMSR(writeValue, msr.addrPowerLimits); err != nil {
		return err
	}
	newVal, err := readMSR(msr.addrPowerLimits, 0)
	if err != nil {
		return err
	}
	if newVal != writeValue {
		return fmt.Errorf("failed to apply power limit: tried to set 0x%x, read 0x%x", writeValue, newVal)
	}
	return nil
}

// ---------- Utility Functions ----------

func boolToEnabled(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}

// ---------- Systemd Persistence Functions ----------

const persistConfigServiceName = "undervolt-go.service"
const persistConfigServicePath = "/etc/systemd/system/" + persistConfigServiceName

func runSystemctlCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	_ = cmd.Run() // Ignore errors if already stopped/disabled
}

// enablePersistence creates and enables the systemd service based on current flags
func enablePersistence() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not get executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("could not resolve executable path: %w", err)
	}

	// Reconstruct arguments, ignoring --persist
	var execArgs []string
	for _, arg := range os.Args[1:] {
		if arg == "--persist" || arg == "-persist" {
			continue
		}
		// Wrap in quotes if it contains spaces
		if strings.Contains(arg, " ") {
			execArgs = append(execArgs, fmt.Sprintf("%q", arg))
		} else {
			execArgs = append(execArgs, arg)
		}
	}

	execStart := exePath
	if len(execArgs) > 0 {
		execStart += " " + strings.Join(execArgs, " ")
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=Apply Undervolt Go settings on boot and resume
After=multi-user.target suspend.target hibernate.target hybrid-sleep.target suspend-then-hibernate.target

[Service]
Type=oneshot
ExecStart=%s

[Install]
WantedBy=multi-user.target suspend.target hibernate.target hybrid-sleep.target suspend-then-hibernate.target
`, execStart)

	fmt.Printf("\nCreating systemd service at %s...\n", persistConfigServicePath)
	if err := os.WriteFile(persistConfigServicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	runSystemctlCmd("systemctl", "daemon-reload")
	runSystemctlCmd("systemctl", "enable", persistConfigServiceName)

	fmt.Println("Persistence enabled successfully. Settings will automatically apply on boot and wake.")
	return nil
}

// disablePersistence removes the systemd service entirely
func disablePersistence() error {
	fmt.Printf("Removing systemd service %s...\n", persistConfigServicePath)

	runSystemctlCmd("systemctl", "stop", persistConfigServiceName)
	runSystemctlCmd("systemctl", "disable", persistConfigServiceName)

	if err := os.Remove(persistConfigServicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	runSystemctlCmd("systemctl", "daemon-reload")
	runSystemctlCmd("systemctl", "reset-failed")

	fmt.Println("Persistence disabled successfully.")
	return nil
}

// ---------- Cobra Command Setup ----------

var (
	readFlag           bool
	verboseFlag        bool
	forceFlag          bool
	tempFlag           int
	tempBatFlag        int
	turboFlag          int
	coreOffset         float64
	gpuOffset          float64
	cacheOffset        float64
	uncoreOffset       float64
	analogioOffset     float64
	p1Args             []string
	p2Args             []string
	lockPowerLimit     bool
	persistFlag        bool
	disablePersistFlag bool
)

func applyFlags() error {
	// Setup logging.
	if verboseFlag {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetOutput(io.Discard)
	}

	msr := ADDRESSES

	// Apply voltage offsets if provided.
	if !math.IsNaN(coreOffset) {
		if err := setOffset("core", coreOffset, msr, forceFlag); err != nil {
			return err
		}
	}
	if !math.IsNaN(gpuOffset) {
		if err := setOffset("gpu", gpuOffset, msr, forceFlag); err != nil {
			return err
		}
	}
	if !math.IsNaN(cacheOffset) {
		if err := setOffset("cache", cacheOffset, msr, forceFlag); err != nil {
			return err
		}
	}
	if !math.IsNaN(uncoreOffset) {
		if err := setOffset("uncore", uncoreOffset, msr, forceFlag); err != nil {
			return err
		}
	}
	if !math.IsNaN(analogioOffset) {
		if err := setOffset("analogio", analogioOffset, msr, forceFlag); err != nil {
			return err
		}
	}

	// Set temperature targets if provided.
	if tempFlag >= 0 && tempFlag != 0 {
		if err := setTemperature(tempFlag, msr); err != nil {
			return err
		}
	}
	if tempBatFlag >= 0 && tempBatFlag != 0 {
		if err := setTemperature(tempBatFlag, msr); err != nil {
			return err
		}
	}

	// Set turbo state if provided.
	if turboFlag >= 0 {
		path := "/sys/devices/system/cpu/intel_pstate/no_turbo"
		f, err := os.OpenFile(path, os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", path, err)
		}
		state := strconv.Itoa(turboFlag)
		if _, err := f.WriteString(state); err != nil {
			f.Close()
			return fmt.Errorf("failed to write to %s: %w", path, err)
		}
		f.Close()

		if turboFlag == 0 {
			fmt.Println("New Intel Turbo State ENABLED")
		} else {
			fmt.Println("New Intel Turbo State DISABLED")
		}
	}

	// Adjust power limits if specified.
	var pl PowerLimit
	// For long term (P1)
	if len(p1Args) > 0 {
		if len(p1Args) != 2 {
			return fmt.Errorf("P1 requires two arguments: POWER_LIMIT TIME_WINDOW")
		}
		power, err1 := strconv.ParseFloat(p1Args[0], 64)
		timeWin, err2 := strconv.ParseFloat(p1Args[1], 64)
		if err1 != nil || err2 != nil {
			return fmt.Errorf("invalid P1 arguments")
		}
		pl.LongTermEnabled = true
		pl.LongTermPower = power
		pl.LongTermTime = timeWin
	}
	// For short term (P2)
	if len(p2Args) > 0 {
		if len(p2Args) != 2 {
			return fmt.Errorf("P2 requires two arguments: POWER_LIMIT TIME_WINDOW")
		}
		power, err1 := strconv.ParseFloat(p2Args[0], 64)
		timeWin, err2 := strconv.ParseFloat(p2Args[1], 64)
		if err1 != nil || err2 != nil {
			return fmt.Errorf("invalid P2 arguments")
		}
		pl.ShortTermEnabled = true
		pl.ShortTermPower = power
		pl.ShortTermTime = timeWin
	}
	if lockPowerLimit {
		pl.Locked = true
	}

	if len(p1Args) > 0 || len(p2Args) > 0 || lockPowerLimit {
		if err := setPowerLimit(pl, msr); err != nil {
			return err
		}
	}

	// If --read is set, print current settings.
	if readFlag {
		temp, err := readTemperature(msr)
		if err != nil {
			return err
		}
		fmt.Printf("Current Settings:\n\n")
		fmt.Printf("Temperature target: -%d (%d°C)\n", temp, 100-temp)
		fmt.Printf("Voltage Offsets:\n")
		for plane := range planes {
			voltage, err := readOffset(plane, msr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading %s offset: %v\n", plane, err)
				continue
			}
			fmt.Printf("   %s: %.2f mV\n", plane, voltage)
		}
		// Read turbo state.
		path := "/sys/devices/system/cpu/intel_pstate/no_turbo"
		data, err := os.ReadFile(path)
		if err == nil {
			state := "enable"
			if string(data) == "1\n" {
				state = "disable"
			}
			fmt.Printf("Intel Turbo: %s\n", state)
		}
		// Read and print power limits.
		plRead, err := readPowerLimit(msr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading power limits:", err)
		} else {
			fmt.Printf("Power limit:\n   %.2fW [P2 (short): %.2fs - %s]\n   %.2fW [P1 (long): %.2fs - %s]%s\n",
				plRead.ShortTermPower,
				plRead.ShortTermTime,
				boolToEnabled(plRead.ShortTermEnabled),
				plRead.LongTermPower,
				plRead.LongTermTime,
				boolToEnabled(plRead.LongTermEnabled),
				func() string {
					if plRead.Locked {
						return " [locked]"
					}
					return ""
				}())
		}

		// Check persistence status
		fmt.Printf("\nBoot/Resume Persistence Status:\n")
		if _, err := os.Stat(persistConfigServicePath); err == nil {
			fmt.Println("   Status: ENABLED (systemd service active)")
			content, err := os.ReadFile(persistConfigServicePath)
			foundCmd := false
			if err == nil {
				for _, line := range strings.Split(string(content), "\n") {
					line = strings.TrimSpace(line) // Clean up potential whitespace
					if strings.HasPrefix(line, "ExecStart=") {
						fmt.Printf("   Active Command: %s\n", strings.TrimPrefix(line, "ExecStart="))
						foundCmd = true
						break
					}
				}
			}

			// Fallback in case the file exists but is malformed
			if !foundCmd {
				fmt.Println("   Active Command: [Service active, but ExecStart could not be parsed]")
			}
		} else {
			fmt.Println("   Status: DISABLED")
		}
	}
	return nil
}

var rootCmd = &cobra.Command{
	Use:          rootCmdUseString,
	Version:      version,
	Short:        "A tool for undervolting and power limit adjustments",
	Long:         "\nUndervolt Go\n\nA no-dependency utility to undervolt Intel CPUs on Linux systems.\n\nPlease use with extreme caution. It has the potential to damage your computer if used incorrectly.",
	SilenceUsage: true, // Do not print usage when returning an execution error
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Do not require root/MSR for help or list commands
		if cmd.Name() == "help" || cmd.Name() == "list" || cmd.Name() == "save" {
			return nil
		}

		if os.Geteuid() != 0 {
			return fmt.Errorf("you need to have root privileges. Rerun with sudo")
		}

		matches, err := filepath.Glob("/dev/cpu/*/msr")
		if err != nil || len(matches) == 0 {
			if err := exec.Command("modprobe", "msr").Run(); err != nil {
				return fmt.Errorf("failed to load msr module (is it enabled in your kernel?): %w", err)
			}
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().NFlag() == 0 {
			runGUI()
			return nil
		}

		// Handle --disable-persist
		if disablePersistFlag {
			if err := disablePersistence(); err != nil {
				return fmt.Errorf("error disabling persistence: %w", err)
			}
			// Exit early if that was the only flag passed
			if cmd.Flags().NFlag() == 1 {
				return nil
			}
		}

		// Apply the settings
		if err := applyFlags(); err != nil {
			return fmt.Errorf("failed to apply settings: %w", err)
		}

		// Handle --persist
		if persistFlag {
			if err := enablePersistence(); err != nil {
				return fmt.Errorf("error enabling persistence: %w", err)
			}
		}
		return nil
	},
}

func init() {
	// run before any command - load the config file
	cobra.OnInitialize(initConfig)

	// Basic undervolt flags.
	rootCmd.PersistentFlags().BoolVar(&readFlag, "read", false, "Read existing values")
	rootCmd.PersistentFlags().BoolVar(&verboseFlag, "verbose", false, "Print debug information")
	rootCmd.PersistentFlags().BoolVar(&forceFlag, "force", false, "Allow setting positive offsets")
	rootCmd.PersistentFlags().IntVar(&tempFlag, "temp", -1, "Set temperature target on AC (°C)")
	rootCmd.PersistentFlags().IntVar(&tempBatFlag, "temp-bat", -1, "Set temperature target on battery (°C)")
	rootCmd.PersistentFlags().IntVar(&turboFlag, "turbo", -1, "Set Intel Turbo (1 disabled, 0 enabled)")

	// Voltage offset flags.
	rootCmd.PersistentFlags().Float64Var(&coreOffset, "core", math.NaN(), "Core offset (mV)")
	rootCmd.PersistentFlags().Float64Var(&gpuOffset, "gpu", math.NaN(), "GPU offset (mV)")
	rootCmd.PersistentFlags().Float64Var(&cacheOffset, "cache", math.NaN(), "Cache offset (mV)")
	rootCmd.PersistentFlags().Float64Var(&uncoreOffset, "uncore", math.NaN(), "Uncore offset (mV)")
	rootCmd.PersistentFlags().Float64Var(&analogioOffset, "analogio", math.NaN(), "AnalogIO offset (mV)")

	// Power limit flags as string slices for multi-value support.
	rootCmd.PersistentFlags().StringSliceVar(&p1Args, "p1", []string{}, "P1 Power Limit (W) and Time Window (s), e.g., --p1=35,10")
	rootCmd.PersistentFlags().StringSliceVar(&p2Args, "p2", []string{}, "P2 Power Limit (W) and Time Window (s), e.g., --p2=45,5")
	rootCmd.PersistentFlags().BoolVar(&lockPowerLimit, "lock-power-limit", false, "Lock the power limit")

	// Systemd Persistence Flags
	rootCmd.PersistentFlags().BoolVar(&persistFlag, "persist", false, "Create a systemd service to persist current settings")
	rootCmd.PersistentFlags().BoolVar(&disablePersistFlag, "disable-persist", false, "Remove the persistence systemd service")

	rootCmd.AddCommand(profileCmd)
	profileCmd.AddCommand(profileSaveCmd, profileListCmd, profileApplyCmd, profileAutoCmd)
}

/* keep temporary : migration of configs to new location */
// we are changing the config dir to /etc/undervolt-go for uninterrupted systemd service runs
const newConfigDir = "/etc/undervolt-go"
const configFileName = "config.yaml"

// 1. Keep the old logic just to locate the legacy configuration
func oldConfigDir() string {
	// 1) If sudo was used, SUDO_USER tells us who really invoked us
	if su := os.Getenv("SUDO_USER"); su != "" {
		if u, err := user.Lookup(su); err == nil {
			return filepath.Join(u.HomeDir, ".config", "undervolt-go")
		}
	}
	// 2) Otherwise, use the normal user config directory
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "undervolt-go")
	}
	// 3) Fallback to HOME.
	return filepath.Join(os.Getenv("HOME"), ".config", "undervolt-go")
}

// 2. The Migration Function
func migrateConfigIfNeeded() error {
	oldPath := filepath.Join(oldConfigDir(), configFileName)
	newPath := filepath.Join(newConfigDir, configFileName)

	// Step A: Check if the old config exists. If it doesn't, nothing to migrate.
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return nil
	}

	// Step B: Check if the new config ALREADY exists.
	// If it does, we shouldn't overwrite it. We can just delete the old one or leave it alone.
	if _, err := os.Stat(newPath); err == nil {
		return nil
	}

	// Step C: Ensure we have root privileges to write to /etc/
	if os.Geteuid() != 0 {
		return fmt.Errorf("a legacy configuration was found at %s. Please run this command with 'sudo' once to migrate it", oldPath)
	}

	log.Printf("Migrating configuration from %s to %s...", oldPath, newPath)

	// Step D: Create /etc/undervolt-go/
	if err := os.MkdirAll(newConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create /etc/ directory: %w", err)
	}

	// Step E: Copy the file (Prevents 'invalid cross-device link' errors)
	configData, err := os.ReadFile(oldPath)
	if err != nil {
		return fmt.Errorf("failed to read old config: %w", err)
	}

	if err := os.WriteFile(newPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write new config: %w", err)
	}

	// Step F: Clean up the old configuration so we don't migrate again
	if err := os.Remove(oldPath); err != nil {
		log.Printf("Warning: Migrated config to /etc/, but failed to delete old config: %v", err)
	}

	// Attempt to remove the old directory if it's empty
	_ = os.Remove(oldConfigDir())

	log.Println("Migration successful!")
	return nil
}

// 3. Your new simplified configDir function
func configDir() string {
	return newConfigDir
}

// initConfig loads /etc/undervolt-go/config.yaml (if present)
func initConfig() {
	cfg := filepath.Join(configDir(), "config.yaml")
	viper.SetConfigFile(cfg)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		// no log output on boot when file is non-existent
	}
}

// profile subcommand
var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage saved profiles",
}

// save profile subcommand to profile subcommand
var profileSaveCmd = &cobra.Command{
	Use:   "save [ac|battery]",
	Short: "Save current flags as a profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		base := "profiles." + name + "."
		viper.Set(base+"planes.core", coreOffset)
		viper.Set(base+"planes.gpu", gpuOffset)
		viper.Set(base+"planes.cache", cacheOffset)
		viper.Set(base+"planes.uncore", uncoreOffset)
		viper.Set(base+"planes.analogio", analogioOffset)
		viper.Set(base+"tl.temp", tempFlag)
		viper.Set(base+"tl.temp-bat", tempBatFlag)
		viper.Set(base+"turbo", turboFlag)
		// Only save P1 if exactly two args were provided
		if len(p1Args) == 2 {
			p1_0, err1 := strToFloat64(p1Args[0])
			p1_1, err2 := strToFloat64(p1Args[1])
			if err1 != nil || err2 != nil {
				return fmt.Errorf("invalid numeric args for P1")
			}
			viper.Set(base+"pl.p1", []float64{p1_0, p1_1})
		}
		// Only save P2 if exactly two args were provided
		if len(p2Args) == 2 {
			p2_0, err1 := strToFloat64(p2Args[0])
			p2_1, err2 := strToFloat64(p2Args[1])
			if err1 != nil || err2 != nil {
				return fmt.Errorf("invalid numeric args for P2")
			}
			viper.Set(base+"pl.p2", []float64{p2_0, p2_1})
		}

		if err := os.MkdirAll(filepath.Join(configDir()), 0755); err != nil {
			return err
		}
		if err := viper.WriteConfigAs(filepath.Join(configDir(), "config.yaml")); err != nil {
			return fmt.Errorf("error saving config: %w", err)
		}
		fmt.Println("Profile ", name, " saved.")
		return nil
	},
}

// list profiles subcommand to profile subcommand
var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available profiles",
	Run: func(cmd *cobra.Command, args []string) {
		for name := range viper.GetStringMap("profiles") {
			fmt.Println(" -", name)
		}
	},
}

// apply profile subcommand to profile subcommand
var profileApplyCmd = &cobra.Command{
	Use:   "apply [auto|ac|battery]",
	Short: "Apply given profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if name == "auto" {
			if isBatteryDischarging() {
				name = "battery"
			} else {
				name = "ac"
			}
		}
		key := "profiles." + name
		if !viper.IsSet(key) {
			return fmt.Errorf("profile '%s' not found", name)
		}
		// Get values from profile and set the variables
		p := viper.Sub(key)
		coreOffset = p.GetFloat64("planes.core")
		gpuOffset = p.GetFloat64("planes.gpu")
		cacheOffset = p.GetFloat64("planes.cache")
		uncoreOffset = p.GetFloat64("planes.uncore")
		analogioOffset = p.GetFloat64("planes.analogio")
		tempFlag = p.GetInt("tl.temp")
		tempBatFlag = p.GetInt("tl.temp-bat")
		turboFlag = p.GetInt("turbo")
		/*
		 *			we can actually do. the only problem is that the values are ints and the flags are strings
		 *			p1Args := p.GetIntSlice("pl.p1")
		 *			p2Args := p.GetIntSlice("pl.p2")
		 */
		// power‑limit slices:
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
		// Apply the settings
		if err := applyFlags(); err != nil {
			return fmt.Errorf("failed to apply settings: %w", err)
		}
		return nil
	},
}

// profile auto-switch based on battery state. creates the systemd service and udev rule, requiring root privileges.
const autoUdevRule = "/etc/udev/rules.d/99-undervolt-go-auto.rules"
const autoServicePath = "/etc/systemd/system/undervolt-go-auto.service"

var profileAutoCmd = &cobra.Command{
	Use:   "auto-switch [enable|disable]",
	Short: "Enable or disable automatic profile switching on AC/Battery events",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		action := args[0]
		if action == "enable" {
			exePath, _ := os.Executable()
			exePath, _ = filepath.EvalSymlinks(exePath)

			serviceContent := fmt.Sprintf(`[Unit]
Description=Apply Undervolt Go Auto Profile
After=multi-user.target

[Service]
Type=oneshot
ExecStart=%s profile apply auto
`, exePath)

			if err := os.WriteFile(autoServicePath, []byte(serviceContent), 0644); err != nil {
				return fmt.Errorf("failed to create service: %w", err)
			}

			// Dynamically find systemctl path to account for different Linux distros (e.g. NixOS)
			systemctlPath, err := exec.LookPath("systemctl")
			if err != nil {
				systemctlPath = "/usr/bin/systemctl"
			}

			// --no-block is important so udev doesn't hang waiting for the command
			ruleContent := fmt.Sprintf(`SUBSYSTEM=="power_supply", ACTION=="change", RUN+="%s --no-block start undervolt-go-auto.service"`+"\n", systemctlPath)
			if err := os.WriteFile(autoUdevRule, []byte(ruleContent), 0644); err != nil {
				return fmt.Errorf("failed to create udev rule: %w", err)
			}

			exec.Command("systemctl", "daemon-reload").Run()
			exec.Command("udevadm", "control", "--reload-rules").Run()
			fmt.Println("Auto-switch enabled.")

		} else if action == "disable" {
			os.Remove(autoServicePath)
			os.Remove(autoUdevRule)
			exec.Command("systemctl", "daemon-reload").Run()
			exec.Command("udevadm", "control", "--reload-rules").Run()
			fmt.Println("Auto-switch disabled.")
		} else {
			return fmt.Errorf("invalid argument. Use 'enable' or 'disable'")
		}
		return nil
	},
}

func main() {
	// Run migration as early as possible in the initialization
	if err := migrateConfigIfNeeded(); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	if err := rootCmd.Execute(); err != nil {
		// Output handled inherently by cobra RunE structure
		os.Exit(1)
	}
}

// helper functions
// strToFloat64 converts a string to float64 or fatally logs.
func strToFloat64(s string) (float64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric argument %q: %w", s, err)
	}
	return f, nil
}
