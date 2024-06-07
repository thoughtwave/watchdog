
# Watchdog

`watchdog` is a simple TCP-based heartbeat monitoring tool. It can run in either server or client mode, allowing for the monitoring of remote systems and the execution of specified scripts upon heartbeat failures.  It is designed to run in place of a hardware watchdog timer, which may not be available on every platform.  It uses a simple pre-shared key to allow a heartbeat conversation.  Multiple copies of the client can run on a given system and speak to multiple servers monitoring them, for a full mesh of availability.  This was written to ensure uptime availability of OpenBSD edge routers, sometimes in instances where they are virtualized don't necessarily have hardware watchdog timers.

## Features

- **Server Mode:** Listens for heartbeat messages from clients. Executes specified scripts if heartbeats are not received within the configured timeout period.
- **Client Mode:** Sends heartbeat messages to the server at regular intervals.
- **Configuration File:** Allows customization of settings via a configuration file.
- **Logging:** Logs all operations and events to a specified log file.
- **Foreground/Background Execution:** Can run in the foreground for testing or in the background as a daemon.
- **Footprint:** Lightweight and can be run as a non-privileged user if desired
- **Portability:** This has been tested on Linux as well as OpenBSD.
- **Extendability:** When triggered, any number of scripts can be executed in a rc.d style, which can do things like an `ipmitool power cycle`, notify Slack, or pretty much anything that can be scripted.

## Installation

Make sure that you have at least Go 1.11.x available.  This has not been tested on anything older, but should work on 1.11.x.

1. Clone the repository:

```sh
git clone git@github.com:thoughtwave/watchdog.git
cd watchdog
```

2. Build the binary:

```sh
go build -o watchdog
```

3. Install the binary (Linux and *BSD instructions):
```sh
sudo mv watchdog /usr/sbin/watchdog && sudo chmod ugo+x /usr/sbin/watchdog
```

4. Set up the service to run at boot on a server:
- **Linux**: 
  ```sh
  cat > /etc/systemd/system/watchdog.service <<EOF
  [Unit]
  Description=watchdog Service
  Wants=network-online.target
  After=network-online.target

  [Service]
  User=root
  Group=root
  Type=simple
  ExecStart=/usr/sbin/watchdog --server --foreground
  [Install]
  WantedBy=multi-user.target
  EOF

  systemctl daemon-reload

  systemctl enable watchdog

  systemctl start watchdog

  ```

- **OpenBSD**:
  ```sh
  echo "/usr/sbin/watchdog --server" >> /etc/rc.local
  ```

5. Set up the service to run at boot on a client:
- **Linux**: 
  ```sh
  cat > /etc/systemd/system/watchdog-client.service <<EOF
  [Unit]
  Description=watchdog Service
  Wants=network-online.target
  After=network-online.target

  [Service]
  User=root
  Group=root
  Type=simple
  ExecStart=/usr/sbin/watchdog --client --foreground
  [Install]
  WantedBy=multi-user.target
  EOF

  systemctl daemon-reload

  systemctl enable watchdog-client

  systemctl start watchdog-client

  ```

- **OpenBSD**:
  ```sh
  echo "/usr/sbin/watchdog --client" >> /etc/rc.local
  ```

## Usage

### Command Line Options

```
Usage: watchdog --key <key> --server | --client --remote <remote-host> [--port <port>] [--timeout <seconds>] [--dir <directory>] [--logs <logfile>] [--foreground] [--attempts <number>] [--config <config-file>]
```

- `--key <key>`: Key for authentication (mandatory).
- `--server`: Run in server mode.
- `--client`: Run in client mode.
- `--remote <remote-host>`: Remote host to connect to (for client mode).
- `--port <port>`: Port to use (default 4848).
- `--timeout <seconds>`: Timeout in seconds (default 600).
- `--dir <directory>`: Directory with scripts to run (default /etc/watchdog.d/).
- `--logs <logfile>`: Log file (default /var/log/watchdog.log).
- `--foreground`: Run in foreground.
- `--attempts <number>`: Number of failed attempts before running scripts (default 3).
- `--config <config-file>`: Configuration file to use (default /etc/watchdog.conf).

### Example

#### Server Mode

```sh
./watchdog --key mysecretkey --server --port 4848 --timeout 600 --dir /etc/watchdog.d/ --logs /var/log/watchdog.log --foreground
```

#### Client Mode

```sh
./watchdog --key mysecretkey --client --remote watchdog-server.example.com --port 4848 --timeout 600 --logs /var/log/watchdog.log --foreground
```

### Configuration File

The configuration file (`/etc/watchdog.conf` by default) can be used to specify the settings for `watchdog`. Example configuration:

```
port 4848
timeout 600
dir /etc/watchdog.d/
logs /var/log/watchdog.log
foreground false
attempts 3
```

## How It Works

- **Server Mode:**
  - Listens for connections on the specified port.
  - Validates incoming messages against the provided key.
  - Updates the last heartbeat timestamp upon receiving a valid heartbeat.
  - Monitors the heartbeat interval and runs specified scripts if the interval exceeds the configured timeout.

- **Client Mode:**
  - Establishes a connection to the server at regular intervals.
  - Sends the specified key as a heartbeat message.
  - Logs the server's response.

## Logging

Logs are written to the specified log file. The log file can be customized via the `--logs` command line option or the configuration file.

## Running in Background

The `--foreground` flag can be used to run `watchdog` in the foreground for testing purposes. By default, `watchdog` runs in the background as a daemon.

## License

This project is licensed under the GPL v2.0 License.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request for any changes.  Please make this better.  Your greatest compliment is using this in production.

## Contact

For any questions or issues, please contact me at [jonathan@thoughtwave.com].

---

This README file provides an overview of the `watchdog` tool, its features, installation instructions, usage examples, and configuration details. Use this as a reference to set up and run `watchdog` in your environment.
