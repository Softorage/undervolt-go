// main.go
// Go port of undervolt.py (v0.4.0) including power limit adjustments using cobra.
// WARNING: Undervolting and power limit changes are dangerous; use at your own risk.

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
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

// assertRoot exits if not run as root.
func assertRoot() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "You need to have root privileges. Rerun with sudo.")
		os.Exit(1)
	}
}

// writeMSR writes an 8-byte little-endian value to the given address on all CPUs.
func writeMSR(val uint64, addr uint64) error {
	assertRoot()
	cpus, err := validCPUs()
	if err != nil {
		return err
	}
	for _, cpu := range cpus {
		path := fmt.Sprintf("/dev/cpu/%d/msr", cpu)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("msr module not loaded (run modprobe msr)")
		}
		log.Printf("Writing 0x%x to %s", val, path)
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		_, err = f.Seek(int64(addr), io.SeekStart)
		if err != nil {
			f.Close()
			return err
		}
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, val)
		if _, err := f.Write(buf); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	return nil
}

// readMSR reads an 8-byte little-endian value from the given address on the specified CPU.
func readMSR(addr uint64, cpu int) (uint64, error) {
	assertRoot()
	path := fmt.Sprintf("/dev/cpu/%d/msr", cpu)
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	_, err = f.Seek(int64(addr), io.SeekStart)
	if err != nil {
		return 0, err
	}
	buf := make([]byte, 8)
	if _, err := io.ReadFull(f, buf); err != nil {
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
func packOffset(planeIndex int, offsetPtr *uint32) uint64 {
	var off uint32 = 0
	var hasOffset uint64 = 0
	if offsetPtr != nil {
		off = *offsetPtr
		hasOffset = 1
	}
	// ((1 << 63) | (planeIndex << 40) | (1 << 36) | (hasOffset << 32) | off)
	return (1 << 63) | (uint64(planeIndex) << 40) | (1 << 36) | (hasOffset << 32) | uint64(off)
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
	valueToWrite := packOffset(planeIndex, nil)
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
	writeValue := packOffset(planeIndex, &target)
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
	pl.ShortTermPower = float64((val >> 32) & 0x7fff) / powerUnit
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

// ---------- Cobra Command Setup ----------

var (
	readFlag       bool
	verboseFlag    bool
	forceFlag      bool
	tempFlag       int
	tempBatFlag    int
	turboFlag      int
	coreOffset     float64
	gpuOffset      float64
	cacheOffset    float64
	uncoreOffset   float64
	analogioOffset float64
	// Use string slices for multi-argument power limit flags.
	p1Args         []string
	p2Args         []string
	lockPowerLimit bool
)

var rootCmd = &cobra.Command{
	Use:   "undervolt-go",
	Version: version,
	Short: "A tool for undervolting and power limit adjustments",
	Long:  "A Go port of undervolt.py (v0.4.0) including power limit adjustments using the cobra library for improved flag handling.",
	Run: func(cmd *cobra.Command, args []string) {
		// Setup logging.
		if verboseFlag {
			log.SetFlags(log.LstdFlags | log.Lshortfile)
		} else {
			log.SetOutput(io.Discard)
		}

		// If no flags are provided, show usage.
		if cmd.Flags().NFlag() == 0 {
			cmd.Usage()
			os.Exit(1)
		}

		// Ensure the MSR module is loaded.
		matches, err := filepath.Glob("/dev/cpu/*/msr")
		if err != nil || len(matches) == 0 {
			cmd := exec.Command("modprobe", "msr")
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load msr module: %v\n", err)
				os.Exit(1)
			}
		}

		msr := ADDRESSES

		// Apply voltage offsets if provided.
		if !math.IsNaN(coreOffset) {
			if err := setOffset("core", coreOffset, msr, forceFlag); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		if !math.IsNaN(gpuOffset) {
			if err := setOffset("gpu", gpuOffset, msr, forceFlag); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		if !math.IsNaN(cacheOffset) {
			if err := setOffset("cache", cacheOffset, msr, forceFlag); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		if !math.IsNaN(uncoreOffset) {
			if err := setOffset("uncore", uncoreOffset, msr, forceFlag); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		if !math.IsNaN(analogioOffset) {
			if err := setOffset("analogio", analogioOffset, msr, forceFlag); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}

		// Set temperature targets if provided.
		if tempFlag >= 0 && tempFlag != 0 {
			if err := setTemperature(tempFlag, msr); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		if tempBatFlag >= 0 && tempBatFlag != 0 {
			if err := setTemperature(tempBatFlag, msr); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}

		// Set turbo state if provided.
		if turboFlag >= 0 {
			path := "/sys/devices/system/cpu/intel_pstate/no_turbo"
			f, err := os.OpenFile(path, os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open %s: %v\n", path, err)
				os.Exit(1)
			}
			defer f.Close()
			state := strconv.Itoa(turboFlag)
			if _, err := f.WriteString(state); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write to %s: %v\n", path, err)
				os.Exit(1)
			}
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
				fmt.Fprintln(os.Stderr, "p1 requires two arguments: POWER_LIMIT TIME_WINDOW")
				os.Exit(1)
			}
			power, err1 := strconv.ParseFloat(p1Args[0], 64)
			timeWin, err2 := strconv.ParseFloat(p1Args[1], 64)
			if err1 != nil || err2 != nil {
				fmt.Fprintln(os.Stderr, "invalid p1 arguments")
				os.Exit(1)
			}
			pl.LongTermEnabled = true
			pl.LongTermPower = power
			pl.LongTermTime = timeWin
		}
		// For short term (P2)
		if len(p2Args) > 0 {
			if len(p2Args) != 2 {
				fmt.Fprintln(os.Stderr, "p2 requires two arguments: POWER_LIMIT TIME_WINDOW")
				os.Exit(1)
			}
			power, err1 := strconv.ParseFloat(p2Args[0], 64)
			timeWin, err2 := strconv.ParseFloat(p2Args[1], 64)
			if err1 != nil || err2 != nil {
				fmt.Fprintln(os.Stderr, "invalid p2 arguments")
				os.Exit(1)
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
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}

		// If --read is set, print current settings.
		if readFlag {
			temp, err := readTemperature(msr)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Printf("temperature target: -%d (%d°C)\n", temp, 100-temp)
			for plane := range planes {
				voltage, err := readOffset(plane, msr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error reading %s offset: %v\n", plane, err)
					continue
				}
				fmt.Printf("%s: %.2f mV\n", plane, voltage)
			}
			// Read turbo state.
			path := "/sys/devices/system/cpu/intel_pstate/no_turbo"
			data, err := os.ReadFile(path)
			if err == nil {
				state := "enable"
				if string(data) == "1\n" {
					state = "disable"
				}
				fmt.Printf("turbo: %s\n", state)
			}
			// Read and print power limits.
			plRead, err := readPowerLimit(msr)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error reading power limits:", err)
			} else {
				fmt.Printf("powerlimit: %.2fW (short: %.2fs - %s) / %.2fW (long: %.2fs - %s)%s\n",
					plRead.ShortTermPower,
					plRead.ShortTermTime,
					boolToEnabled(plRead.ShortTermEnabled),
					plRead.LongTermPower,
					plRead.LongTermTime,
					boolToEnabled(plRead.LongTermEnabled),
					func() string { if plRead.Locked { return " [locked]" } else { return "" } }())
			}
		}
	},
}

func init() {
	// Basic undervolt flags.
	rootCmd.PersistentFlags().BoolVar(&readFlag, "read", false, "read existing values")
	rootCmd.PersistentFlags().BoolVar(&verboseFlag, "verbose", false, "print debug info")
	rootCmd.PersistentFlags().BoolVar(&forceFlag, "force", false, "allow setting positive offsets")
	rootCmd.PersistentFlags().IntVar(&tempFlag, "temp", -1, "set temperature target on AC (°C)")
	rootCmd.PersistentFlags().IntVar(&tempBatFlag, "temp-bat", -1, "set temperature target on battery (°C)")
	rootCmd.PersistentFlags().IntVar(&turboFlag, "turbo", -1, "set Intel Turbo (1 disabled, 0 enabled)")

	// Voltage offset flags.
	rootCmd.PersistentFlags().Float64Var(&coreOffset, "core", math.NaN(), "core offset (mV)")
	rootCmd.PersistentFlags().Float64Var(&gpuOffset, "gpu", math.NaN(), "gpu offset (mV)")
	rootCmd.PersistentFlags().Float64Var(&cacheOffset, "cache", math.NaN(), "cache offset (mV)")
	rootCmd.PersistentFlags().Float64Var(&uncoreOffset, "uncore", math.NaN(), "uncore offset (mV)")
	rootCmd.PersistentFlags().Float64Var(&analogioOffset, "analogio", math.NaN(), "analogio offset (mV)")

	// Power limit flags as string slices for multi-value support.
	rootCmd.PersistentFlags().StringSliceVar(&p1Args, "p1", []string{}, "P1 Power Limit (W) and Time Window (s), e.g., --p1=35,10")
	rootCmd.PersistentFlags().StringSliceVar(&p2Args, "p2", []string{}, "P2 Power Limit (W) and Time Window (s), e.g., --p2=45,5")
	rootCmd.PersistentFlags().BoolVar(&lockPowerLimit, "lock-power-limit", false, "lock the power limit")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
