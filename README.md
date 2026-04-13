# Pick-0-Ball

Pick-0-Ball was developed as part of the National University of Singapore (NUS) CG4002 Capstone Project by Group B03.

This is the single repository for the complete capstone system:

- Unity AR visualiser and gameplay loop
- AI training and Ultra96 FPGA inference
- Embedded firmware for IMU and UWB sensing
- Communications and coordination services

## System Overview

The system is a real-time loop between sensors, inference, and AR rendering.

1. Paddle IMU and button packets are published over MQTT.
2. Player and ball state are processed by Unity and Ultra96 services.
3. AI inference predicts the opponent return.
4. Unity renders the return trajectory and game state.

High-level flow:

```text
ESP32 IMU/UWB --> MQTT broker --> Unity Visualiser
                   |            |
                   v            |
            Communications ---->|
                   |
                   v
            Ultra96 AI (FPGA)
```

## Repository Structure

The repository is organised into four product folders, with supporting repository folders.

| Path | Purpose | Entry point |
|---|---|---|
| `AI/accelerator` | Model training, quantisation, deployment scripts, and Ultra96 runtime tooling | [AI/accelerator/training/train.py](AI/accelerator/training/train.py) |
| `AI/hls` | HLS implementation and Vivado/Vitis projects | [AI/hls/pickleball_model.cpp](AI/hls/pickleball_model.cpp) |
| `Visualiser` | Unity AR client, gameplay, MQTT integration, and game logic | [Visualiser/Assets/Scenes/MainScene.unity](Visualiser/Assets/Scenes/MainScene.unity) |
| `hardware/imu` | IMU firmware and motion processing | [hardware/imu/main_code.ino](hardware/imu/main_code.ino) |
| `hardware/uwb` | UWB firmware and player position processing | [hardware/uwb/UWB_sensor.ino](hardware/uwb/UWB_sensor.ino) |
| `communications` | Go services for SSH tunnel, coordination, and metrics | [communications/main.go](communications/main.go) |
| `.github` | Pull request templates and repository standards | [.github/PULL_REQUEST_TEMPLATE.md](.github/PULL_REQUEST_TEMPLATE.md) |

## Key Technical References

- Architecture reference: [Visualiser/Docs/System_Architecture.md](Visualiser/Docs/System_Architecture.md)
- UML and state diagrams: [Visualiser/Docs/UML_Diagrams.md](Visualiser/Docs/UML_Diagrams.md)
- Script and message reference: [Visualiser/Docs/AI_Agent_Reference.md](Visualiser/Docs/AI_Agent_Reference.md)

## Quick Start By Area

### Visualiser

1. Open `Visualiser` in Unity 6000.4.0f1.
2. Load [Visualiser/Assets/Scenes/MainScene.unity](Visualiser/Assets/Scenes/MainScene.unity).
3. Configure broker endpoints used by scripts in [Visualiser/Assets/Scripts](Visualiser/Assets/Scripts).

### AI Accelerator

1. Use scripts in [AI/accelerator/training](AI/accelerator/training) for dataset preparation and training.
2. Start from [AI/accelerator/training/train.py](AI/accelerator/training/train.py).
3. Use [AI/accelerator/ultra96_deploy](AI/accelerator/ultra96_deploy) for Ultra96 runtime deployment.
4. Operational Telegram bot: [AI/accelerator/ultra96_deploy/telegram_bot.py](AI/accelerator/ultra96_deploy/telegram_bot.py).

### Ultra96 Health Check via Telegram Bot

Use the Telegram bot to confirm whether the Ultra96 is online and responsive.

1. On Ultra96, set the bot token environment variable.
2. Start the bot script from [AI/accelerator/ultra96_deploy/telegram_bot.py](AI/accelerator/ultra96_deploy/telegram_bot.py).
3. In Telegram, send /ping to the bot.

Expected healthy response:

- FPGA Board Status: ONLINE
- Hostname, uptime, CPU temperature, memory, disk, and board power rails

If /ping does not return a response, treat the board as unavailable until proven otherwise. Common causes are power loss, network path issues, bot process not running, or an invalid token.

Other useful operational commands:

- /memtop to inspect top memory consumers
- /cleanup to kill stray PYNQ processes and free memory
- /clearmem to drop caches and compact memory
- /eval_sw and /eval_hw to trigger evaluation scripts remotely

### HLS and FPGA

1. HLS top function: [AI/hls/pickleball_model.cpp](AI/hls/pickleball_model.cpp).
2. Vitis project: [AI/hls/pickleball_hls/hls.app](AI/hls/pickleball_hls/hls.app).
3. Vivado project: [AI/hls/Pickleball_vivado/Pickleball_vivado.xpr](AI/hls/Pickleball_vivado/Pickleball_vivado.xpr).

### Hardware Firmware

1. IMU entry point: [hardware/imu/main_code.ino](hardware/imu/main_code.ino).
2. UWB entry point: [hardware/uwb/UWB_sensor.ino](hardware/uwb/UWB_sensor.ino).

### Communications Service

From [communications](communications), run:

```bash
go run .
```

Primary files:

- [communications/main.go](communications/main.go)
- [communications/system-coordinator.go](communications/system-coordinator.go)
- [communications/network-metrics.go](communications/network-metrics.go)

## Core MQTT Topics

| Topic | Producer | Consumer | Purpose |
|---|---|---|---|
| `/paddle` | ESP32 | Unity | IMU payload and button events |
| `/playerBall` | Unity | Ultra96 path | Ball state after player hit |
| `/opponentBall` | Ultra96 path | Unity | Predicted opponent return state |
| `/playerPosition` | UWB pipeline | Unity | Player position for drift correction |
| `/hitAck` | Unity | ESP32 | Haptic feedback trigger |

## Security Removals

The following sensitive files were removed from source control for security reasons:

- Visualiser/Assets/StreamingAssets/ca.crt
- Visualiser/Assets/StreamingAssets/ca.crt.meta
- Visualiser/Assets/StreamingAssets/unity-client.pfx
- Visualiser/Assets/StreamingAssets/unity-client.pfx.meta
- Visualiser/Assets/StreamingAssets/unity-client.pfx.bytes
- Visualiser/Assets/StreamingAssets/unity-client.pfx.bytes.meta
- AI/accelerator/comms/certs/ca.crt
- AI/accelerator/comms/certs/u96-client.crt
- AI/accelerator/comms/certs/u96-client.key

Embedded certificate and key material was also removed from:

- [hardware/imu/config.h](hardware/imu/config.h)
- [hardware/uwb/config.h](hardware/uwb/config.h)

## Attribution and Team Credits

This project is released under the MIT licence in [LICENSE](LICENSE).

If you use this codebase in coursework, research, or production systems, retain the licence notice and credit the project team.

Recommended credit line:

Pick-0-Ball Team, NUS CG4002 Capstone Project Group B03 (2026): Goh Sze Kang, Dao Trong Khanh, Claribel Ho Jia Huan, Ng Chee Fong.

### Team Roles

| Name | Primary role | GitHub |
|---|---|---|
| Goh Sze Kang | Hardware Sensor Systems Engineering | [gskang-22](https://github.com/gskang-22) |
| Dao Trong Khanh | Software Visualisation Engineering | [tkhahns](https://github.com/tkhahns) |
| Claribel Ho Jia Huan | Communications Systems Engineering | [claribelho](https://github.com/claribelho) |
| Ng Chee Fong | Artificial Intelligence Software and Hardware Engineering | [NCF3535](https://github.com/NCF3535) |
