# NerdCAN

A terminal-based CAN bus monitoring and interaction tool built with Go and Bubble Tea.

NerdCAN provides a clean, interactive command-line interface for sending and receiving CAN messages, with features like message filtering, logging, and persistence.

## Features

- **Real-time CAN Message Monitoring**: View incoming and outgoing CAN messages in a live, updating table.
- **Manual and Cyclic Message Sending**: Send single CAN frames or configure messages for cyclic transmission.
- **Message Persistence**: Save and load your configured send messages to `messages.json`.
- **Filtering**: Filter received messages by ID using whitelist or blacklist modes.
- **Overwrite/Log Modes**: Choose to overwrite existing messages in the display or log all incoming messages.
- **Intuitive TUI**: Navigate and interact with the application using keyboard shortcuts.
- **Bus Load Monitoring**: (Planned/Future) Monitor the CAN bus load.

## Installation

### Prerequisites

- **Go**: Ensure you have Go (version 1.20 or higher recommended) installed on your system.
- **Linux with SocketCAN**: NerdCAN relies on SocketCAN, which is a standard CAN interface for Linux. You'll need to ensure your CAN interface is set up and active.

  **Example SocketCAN Setup (for a `can0` interface):**
  ```bash
  sudo modprobe can
  sudo modprobe can_raw
  sudo ip link set can0 up type can bitrate 500000 # Adjust bitrate as needed
  # You might use `vcan0` for testing without physical hardware:
  # sudo modprobe vcan
  # sudo ip link add dev vcan0 type vcan
  # sudo ip link set up vcan0
  ```

### Building from Source

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-username/nerdcan.git # Replace with your actual repo URL
    cd nerdcan
    ```

2.  **Build the application:**
    ```bash
    go build .
    ```

    This will create an executable named `nerdcan` (or `nerdcan.exe` on Windows) in your current directory.

## Usage

To run NerdCAN, simply execute the built binary:

```bash
./nerdcan
```

### Keybindings

-   `q` or `ctrl+c`: Quit the application.
-   `?`: Toggle help view.
-   `o`: Toggle receive panel mode (overwrite/log).
-   `f`: Cycle through filter modes (Off, Whitelist, Blacklist).
-   `F`: Add/remove selected message ID to/from the current filter list.
-   `esc`: Clear all received messages.
-   `tab`: Switch focus between the receive and send panels.
-   `n`: Create a new message in the send panel.
-   `e`: Edit the currently selected message in the send panel.
-   `d`: Delete the currently selected message from the send panel.
-   `space`: Send the selected message (manual or start/stop cyclic).
-   `ctrl+s`: Save all send messages to `messages.json`.
-   `ctrl+l`: Load send messages from `messages.json`.
-   `ctrl+d`: Clear all send messages.
-   `i`: Toggle info panel (for bus load monitoring, etc.).

## Contributing

Contributions are welcome! Feel free to open issues or submit pull requests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. (Note: You might need to create a LICENSE file if you haven't already.)
