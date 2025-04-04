# undervolt-go

**undervolt-go** is a Go port of the original [undervolt](https://github.com/georgewhewell/undervolt) utility, designed to allow users to undervolt Intel CPUs on Linux systems. Undervolting can help reduce CPU temperatures, decrease power consumption, and potentially increase system stability and longevity. **undervolt-go** gives the advantage of running the application without the need for any dependencies.

_**Note:**_
- *Please use this software with extreme caution. It has the potential to damage your computer if used incorrectly.*

## Table of Contents

- [Introduction](#introduction)
- [Installation](#installation)
- [Building](#building)
- [Usage](#usage)
- [Features](#features)
- [Dependencies](#dependencies)
- [Configuration](#configuration)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)
- [FAQ](#faq)
- [Contributors](#contributors)
- [License](#license)

## Introduction

**undervolt-go** enables users to apply voltage offsets to various components of Intel CPUs, such as the core, cache, GPU, and more. By adjusting these voltage offsets, users can achieve lower power consumption and reduced heat output, which is particularly beneficial for laptops and compact systems where thermal management is crucial.

## Building

To build **undervolt-go**, follow these steps:

1. **Clone the repository:**

   ```bash
   git clone https://github.com/Softorage/undervolt-go.git
   ```

2. **Navigate to the project directory:**

   ```bash
   cd undervolt-go
   ```

3. **Build the application:**

   ```bash
   go build
   ```

   This will generate the `undervolt-go` executable in the current directory.

4. **Run the application:**

   ```bash
   sudo ./undervolt-go --help
   sudo ./undervolt-go --read
   ```

   Because the program accesses MSRs, you must run it as root (e.g., with sudo).


## Installation

To install **undervolt-go** on your system, follow these steps:
1. Download latest release from [offical nightly builds](https://softorage.github.io/undervolt-go/)
2. Extract the archive. You should now have the following files:
   1. undervolt-go
   2. install-undervolt.sh
   3. update-undervolt.sh
3. Simply make install-undervolt.sh executable (or update-undervolt.sh if you already have it):
   - `chmod +x install-undervolt.sh`
   - or you can right click install-undervolt.sh, go to Properties, and in the Permissions tab, tick 'Make executable'
4. If you have built the binary by yourselves, replace the downloaded undervolt-go with your undervolt-go
5. Run install-undervolt.sh (or update-undervolt.sh) with sudo (it's always recommended to check the script by opening it in a text editor before executing it)
   `sudo ./install-undervolt.sh`

## Usage

**undervolt-go** requires root privileges to interact with the CPU's model-specific registers (MSRs). Ensure you have the necessary permissions before proceeding.

1. To apply a voltage offset, use the following syntax:
  
   ```bash
   sudo undervolt-go --core=-100 --cache=-50 --gpu=-50
   ```
   
   This command applies a -100 mV offset to the CPU core, -50 mV to the CPU cache, and a -50 mV offset to the GPU.

2. This command applies a 40W power limit to PL1 and a 32s time window. PL1 is the long term power limit, that can be safe for longer periods.
   
   ```bash
   sudo undervolt-go --p1=40,32
   ```

3. This command applies a 60W power limit to PL2 and a 32s time window. PL2 is the short term power limit, that can be safe for shorter periods and is useful for short bursts of performance.
   
   ```bash
   sudo undervolt-go --p2=60,32
   ```

4. All commands can be found in the help menu:

   ```bash
   Usage:
   undervolt-go [flags]

   Flags:
         --analogio float     analogio offset (mV) (default NaN)
         --cache float        cache offset (mV) (default NaN)
         --core float         core offset (mV) (default NaN)
         --force              allow setting positive offsets
         --gpu float          gpu offset (mV) (default NaN)
   -h, --help               help for undervolt-go
         --lock-power-limit   lock the power limit
         --p1 strings         P1 Power Limit (W) and Time Window (s), e.g., --p1=35,10
         --p2 strings         P2 Power Limit (W) and Time Window (s), e.g., --p2=45,5
         --read               read existing values
         --temp int           set temperature target on AC (°C) (default -1)
         --temp-bat int       set temperature target on battery (°C) (default -1)
         --turbo int          set Intel Turbo (1 disabled, 0 enabled) (default -1)
         --uncore float       uncore offset (mV) (default NaN)
         --verbose            print debug info
   -v, --version            version for undervolt-go
   ```

## Features

- **Voltage Offset Adjustment:** Apply custom voltage offsets to CPU components to optimize performance and thermal characteristics.
- **Temperature Target Override:** Set a custom temperature target for CPU throttling.
- **Power Limit Configuration:** Adjust the CPU's power limits to control performance and power consumption.

## Dependencies

- **For usage**
  - **No dependencies are required.**
  - **Linux Kernel with MSR Support:** Ensure that the `msr` kernel module is loaded. You can load it using:

    ```bash
    sudo modprobe msr
    ```
  - **Root Privileges:** Ensure you have root privileges to interact with the CPU's model-specific registers (MSRs).
- **For building**
  - **Go Programming Language:** Required for building the application. Download and install from the [official Go website](https://golang.org/dl/).

## Configuration

**undervolt-go** does not use a configuration file. All settings are applied via command-line arguments. To maintain settings across reboots, consider creating a startup script that runs your preferred `undervolt-go` command.

## Examples

- **Read Current Voltage Offsets:**

  ```bash
  sudo undervolt-go --read
  ```

  This command displays the current voltage offsets applied to the CPU components.

- **Set Temperature Target to 85°C:**

  ```bash
  sudo undervolt-go --temp=85
  ```

  This sets the CPU throttling temperature target to 85 degrees Celsius.

- **Disable Intel Turbo Boost:**

  ```bash
  sudo undervolt-go --turbo=1
  ```

  This command disables Intel Turbo Boost, potentially reducing heat and power consumption.

## Troubleshooting

- **System Instability:** Applying too much voltage offset can cause system instability or crashes. If you experience issues, reduce the magnitude of the offsets.
- **Settings Reset After Reboot:** Voltage offsets are not persistent across reboots by default. Create a startup script to apply your preferred settings automatically.
- **Permission Denied Errors:** Ensure you are running the commands with `sudo` to have the necessary privileges.

## FAQ

1. Is undervolting safe?

Undervolting can be safe if done correctly, but it carries inherent risks. Applying incorrect voltage settings can lead to system instability, crashes, or hardware damage. Start with small negative voltage offsets, and choose the offset just before the system starts crashing. You may have to manually cut the power when the system crashes due to undervolt.

2. Do I need to install any dependencies to use undervolt-go?

No, undervolt-go is built using Go and does not require any external dependencies. However, to interact with CPU settings, the application needs to run with elevated privileges.

3. How do I use undervolt-go to undervolt my CPU?

To use undervolt-go:

   Mehtod 1: Without installation:

   - Open Terminal: Launch your terminal application.​
   - Navigate to the Executable Location: Ensure you're in the directory containing the undervolt-go executable (`cd` into the directory where 'undervolt-go' is located) or provide the full path to it.​
   - Run the Executable: Execute the program with appropriate permissions:​
      `sudo ./undervolt-go`

   Mehtod 2: With installation:

   - Open Terminal: Launch your terminal application.
   - Navigate to the undervolt-go directory: Change to the directory containing the undervolt-go executable, along with the install-undervolt.sh and update-undervolt.sh scripts. Make sure that all the files are in the same directory.
   - Install undervolt-go: Run the install-undervolt.sh script: 
      `sudo ./install-undervolt.sh`
   - Run undervolt-go: You can now use 'undervolt-go' from any directory.Run the undervolt-go executable with root privileges:
      `sudo undervolt-go`

4. Do you use AI to develop this project?

We may use AI when developing this project. If you find any issues, please report them to us. We will try to fix them as soon as possible.

5. Which Intel CPUs are supported by undervolt-go?

undervolt-go supports a range of Intel CPUs, particularly those from the Haswell generation and newer. However, compatibility can vary based on your specific system configuration. 

## Contributors

We welcome contributions from the community. If you'd like to contribute to **undervolt-go**, please fork the repository and submit a pull request with your changes.

## License

This project is licensed under the GNU General Public License v3.0. See the [LICENSE.txt](LICENSE.txt) file for details.

### NO WARRANTY

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
