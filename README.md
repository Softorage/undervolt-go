# Undervolt Go

**Undervolt Go** is a power-utility, designed to allow users to undervolt Intel CPUs on Linux systems. Undervolting can help reduce CPU temperatures, decrease power consumption, and potentially increase system stability and longevity. **Undervolt Go** gives the advantage of running the application without the need for any dependencies, and also features a user-friendly graphical version.

Get it [here](https://softorage.github.io/undervolt-go/).

_**Note by dev:**_
- *Please use this software with extreme caution. It has the potential to damage your computer if used incorrectly.*
- The tool is tested by me for my personal use and works pretty nice. If you find any issues, please let me know via GitHub Issues.

---

Please leave a star if you find it useful!

---

## Table of Contents

- [Introduction](#introduction)
- [Screenshots](#screenshots)
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
- [Credits](#credits)
- [Other tools](#other-tools)
- [What is Softorage?](#what-is-softorage)
- [License](#license)

## Introduction

**Undervolt Go** enables users to apply voltage offsets to various components of Intel CPUs, such as the core, cache, GPU, and more. By adjusting these voltage offsets, users can achieve lower power consumption and reduced heat output, which is particularly beneficial for laptops and compact systems where thermal management is crucial.

## Building

To build **Undervolt Go**, follow these steps:

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


## Screenshots
| Description          | Screenshot                                                                       |
| -------------------- | -------------------------------------------------------------------------------- |
| Voltage Offsets      | ![Voltage Offsets](/dist/images/screenshots/v0.8.2/VoltOffset-UndervoltGo.png)   |
| Power Limit          | ![Power Limit](/dist/images/screenshots/v0.8.2/PowerLimit-UndervoltGo.png)       |
| Temperature Limit    | ![Temperature Limit](/dist/images/screenshots/v0.8.2/TempLimit-UndervoltGo.png)  |
| Other Flags          | ![Other Flags](/dist/images/screenshots/v0.8.2/OtherFlags-UndervoltGo.png)       |
| Check Core Temps     | ![Check Core Temps](/dist/images/screenshots/v0.8.2/CheckTemp-UndervoltGo.png)   |
| Check Fan RPMs       | ![Check Fan RPMs](/dist/images/screenshots/v0.8.2/CheckFans-UndervoltGo.png)     |
| Read                 | ![Read](/dist/images/screenshots/v0.8.2/Read-UndervoltGo.png)                    |
| Log                  | ![Profiles](/dist/images/screenshots/v0.8.2/Log-UndervoltGo.png)                 |


## Installation

1. Download latest release from [offical nightly builds](https://softorage.github.io/undervolt-go/).
   - You can download the Graphical Interface version (Pro version) or the CLI version.
   - The GUI version can also run the CLI commands. The commands need to be passed to `undervolt-go-pro`
2. Extract the archive. You should now have the following files:
   1. `undervolt-go` or `undervolt-go-pro`
   2. `install-undervolt.sh`
   3. `uninstall-undervolt.sh`
   4. `update-undervolt.sh`
   5. `icon.png` (in case of Pro version)
3. Open Terminal and navigate to the directory containing the undervolt-go or undervolt-go-pro executable. You can often simply launch Terminal in the active folder with a right-click.
4. To install:
   1. Simply make `install-undervolt.sh` executable:
      - `chmod +x install-undervolt.sh`
      - or you can right click `install-undervolt.sh`, go to Properties, and in the Permissions tab, tick 'Make executable'
   2. If you have built the binary by yourselves, replace the downloaded undervolt-go with your undervolt-go (or undervolt-go-pro for that matter)
   3. Run `install-undervolt.sh` with sudo (it's always recommended to check the script by opening it in a text editor before executing it)
      `sudo ./install-undervolt.sh`
5. To update (if you already have Undervolt Go installed on your system):
   1. Simply make `update-undervolt.sh` executable:
      - `chmod +x update-undervolt.sh`
      - or you can right click `update-undervolt.sh`, go to Properties, and in the Permissions tab, tick 'Make executable'
   2. If you have built the binary by yourselves, replace the downloaded undervolt-go with your undervolt-go (or undervolt-go-pro for that matter)
   3. Run `update-undervolt.sh` with sudo (it's always recommended to check the script by opening it in a text editor before executing it)
      `sudo ./update-undervolt.sh`
6. To uninstall:
   1. Simply make `uninstall-undervolt.sh` executable:
      - `chmod +x uninstall-undervolt.sh`
      - or you can right click `uninstall-undervolt.sh`, go to Properties, and in the Permissions tab, tick 'Make executable'
   2. If you have built the binary by yourselves, replace the downloaded undervolt-go with your undervolt-go (or undervolt-go-pro for that matter)
   3. Run `uninstall-undervolt.sh` with sudo (it's always recommended to check the script by opening it in a text editor before executing it)
      - Completely uninstall, including configuration files, systemd services for persist and profile auto-swtich, udev rule for profile auto-switch
         `sudo ./uninstall-undervolt.sh`
      - Keep configuration files, systemd services for persist and profile auto-swtich, udev rule for profile auto-switch
         `sudo ./uninstall-undervolt.sh --keepconfig --keeppersist --keepautoswitch`

## Usage

**Undervolt Go** requires root privileges to interact with the CPU's model-specific registers (MSRs). Ensure you have the necessary permissions before proceeding.

1. To apply a voltage offset, use the following syntax:
  
   ```bash
   sudo undervolt-go --core=-100 --cache=-50 --gpu=-50
   ```
   
   This command applies a -100 mV offset to the CPU core, -50 mV to the CPU cache, and a -50 mV offset to the GPU.

2. This command applies a 40W power limit to PL1 and a 32s time window. PL1 is the long term power limit, that can be safe for longer periods.
   
   ```bash
   sudo undervolt-go --p1=40,32
   ```

3. This command applies a 60W power limit to PL2 and a 10s time window. PL2 is the short term power limit, that can be safe for shorter periods and is useful for short bursts of performance.
   
   ```bash
   sudo undervolt-go --p2=60,10
   ```

4. You can use multiple flags in a single command.
   
   ```bash
   sudo undervolt-go --core=-70 --cache=-50 --p1=40,32 --p2=60,10 --turbo=0 --temp=78 --temp-bat=66
   ```

5. To persist a configuration across reboots.
   
   ```bash
   sudo undervolt-go --core=-70 --cache=-50 --p1=40,32 --p2=60,10 --turbo=0 --temp=78 --temp-bat=66 --persist
   ```

6. To delete persisted configuration.
  
   ```bash
   sudo undervolt-go --disable-persist
   ```

6. Auto-switching profile based on battery state (charging/discharging).
   - Make sure that `ac` and `battery` profiles exist.
  
      ```bash
      # save ac profile
      sudo undervolt-go profile save ac --core=-50 --cache=-30 --p1=60,32 --p2=80,10 --turbo=0 --temp=84 --temp-bat=66
      
      # save battery profile
      sudo undervolt-go profile save battery --core=-70 --cache=-50 --p1=15,32 --p2=40,10 --turbo=1 --temp=78 --temp-bat=66
      ```
   - Enable auto-switch:

      ```bash
      sudo undervolt-go profile auto-switch enable
      ```
   - Disable auto-switch:
      ```bash
      sudo undervolt-go profile disable
      ```

7. All commands can be found in the help menu:

   ```
   sudo undervolt-go --help

    Undervolt Go

    A no-dependency utility to undervolt Intel CPUs on Linux systems with voltage offsets, perform power limit adjustments, set temperature limits, and more. It also features a user-friendly graphical version which lets you monitor temperatures and fan speeds with the help of 'sensors' package.

    Please use with extreme caution. It has the potential to damage your computer if used incorrectly.

    Usage:
      undervolt-go [flags]
      undervolt-go [command]

    Available Commands:
      completion  Generate the autocompletion script for the specified shell
      help        Help about any command
      profile     Manage saved profiles

    Flags:
          --analogio float     AnalogIO offset (mV) (default NaN)
          --cache float        Cache offset (mV) (default NaN)
          --core float         Core offset (mV) (default NaN)
          --disable-persist    Remove the persistence systemd service completely
          --force              Allow setting positive offsets
          --gpu float          GPU offset (mV) (default NaN)
      -h, --help               help for undervolt-go
          --lock-power-limit   Lock the power limit
          --p1 strings         P1 Power Limit (W) and Time Window (s), e.g., --p1=35,10
          --p2 strings         P2 Power Limit (W) and Time Window (s), e.g., --p2=45,5
          --persist            Create a systemd service to persist current settings across boot and resume
          --read               Read existing values
          --temp int           Set temperature target on AC (°C) (default -1)
          --temp-bat int       Set temperature target on battery (°C) (default -1)
          --turbo int          Set Intel Turbo (1 disabled, 0 enabled) (default -1)
          --uncore float       Uncore offset (mV) (default NaN)
          --verbose            Print debug information
      -v, --version            version for undervolt-go

    Use "undervolt-go [command] --help" for more information about a command.

   ```

8. Make sure to use `undervolt-go-pro` instead of `undervolt-go` in terminal in case you have installed the GUI version.

## Features

- **GUI:** Interact with 'Undervolt Go' from a user-friendly graphical user interface.
- **Voltage Offset Adjustment:** Apply custom voltage offsets to CPU components to optimize performance and thermal characteristics.
- **Temperature Target Override:** Set a custom temperature target for CPU throttling, on AC or battery power.
- **Power Limit Configuration:** Adjust the CPU's power limits to control performance and power consumption.
- **Intel Turbo Adjustment:** Enable or disable Intel Turbo for optimal performance.
- **Profile Management:** Save and apply profiles for quick configuration changes.
- **Auto Profile Switching:** Automatically switch to the appropriate profile based on AC or battery power.
- **Temperature Monitoring:** Monitor and display the current temperature of the CPU.
- **Fan Monitoring:** Monitor and display the current fan speed of the CPU.

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

- You can save configuration using the `profile save [ac/battery] --flags` command.
- You can apply configuration using the `profile apply [auto|ac|battery]` command.
- You can also automatically apply saved profiles based on whether the computer is on AC or battery power with `profile auto-switch [enable|disable]`.
- To maintain settings across reboots, you can now use the --persist flag that creates a small systemd service. Make sure that the configuration that you are persisting across boots is a stable configuration: `--core=-70 --cache=-50 --p1=40,32 --p2=60,10 --turbo=0 --temp=78 --temp-bat=66 --persist`

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

2. Do I need to install any dependencies to use Undervolt Go?

   No, Undervolt Go is built using Go and does not require any external dependencies. However, to interact with CPU settings, the application needs to run with elevated privileges.

3. How do I use Undervolt Go to undervolt my CPU?

   To use Undervolt Go:

      Mehtod 1: Without installation:

      - Open Terminal: Launch your terminal application.​
      - Navigate to the Executable Location: Ensure you're in the directory containing the undervolt-go executable (`cd` into the directory where 'undervolt-go' is located) or provide the full path to it.​
      - Run the Executable: Execute the program with appropriate permissions:​
         `sudo ./undervolt-go`

      Mehtod 2: With installation:

      - Navigate to the undervolt-go or undervolt-go-pro directory: Change to the directory containing the undervolt-go executable or undervolt-go-pro executable for graphical version, along with the `install-undervolt.sh`, `uninstall-undervolt.sh` and `update-undervolt.sh` scripts. Make sure that all the files are in the same directory.
      - Open Terminal: Launch your terminal application in the undervolt-go or undervolt-go-pro directory.
      - Install Undervolt Go: Run the `install-undervolt.sh` script: 
         `sudo ./install-undervolt.sh`
      - Run Undervolt Go:
         - For `undervolt-go`, you can now use 'Undervolt Go' from any directory. Run the `undervolt-go` command with root privileges:
            - `sudo undervolt-go --help`
         - For `undervolt-go-pro`, you can now launch 'Undervolt Go' from your desktop. Just click or double-click on the 'Undervolt Go' icon to run the graphical version. You will be prompted root password and then the program will run.

4. Do you use AI to develop this project?

   We may use AI when developing this project. If you find any issues, please report them to us. We will try to fix them as soon as possible.

5. Do you even know how to code?

   Well, kind of. I am fairly confident that I understand the code I maintain (I keep forgetting though). Sometimes, there do appear parts of code (often via LLMs) that work and I don't quite understand how (and I have to ask to understand). But hey, that was the case even in StackOverflow days. I'm pretty dumb in that regard, just not enough to constantly keep messing the code. (-> Sanmay)

6. Which Intel CPUs are supported by Undervolt Go? Does it support iGPU as well?

   Undervolt Go supports a range of Intel CPUs, particularly those from the Haswell generation and newer. However, compatibility can vary based on your specific system configuration. See the list from `undervolt by georgewhewell` [here](https://github.com/georgewhewell/undervolt#hardware-support). This tool may or may not work with iGPU though. See [this](https://github.com/georgewhewell/undervolt/issues/196).

## Contributors

We welcome contributions from the community. If you'd like to contribute to **Undervolt Go**, please fork the repository and submit a pull request with your changes.

## Credits

* [undervolt](https://github.com/georgewhewell/undervolt) for Intel CPUs on Linux
* [Softorage](https://softorage.com)

## Other tools

*Looking for a a simple 7-zip GUI on Linux that has all the advanced features that you are used to? Try our other tool: [7z-GUI-Linux](https://github.com/Softorage/7z-GUI-Linux).*

## What is Softorage?

Softorage is a software discovery platform that takes user trust and safety very seriously. It allows you to get the software on your computer, but with a distinction. Instead of hosting the packages (which involves risks of package manipulation, and is a well known malware vector), it simply links you to the official developer's website. This helps ensure that you get the software package as the original developers intended.

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
