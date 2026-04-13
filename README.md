# Pick-0-Ball

Pick-0-Ball was developed as part of the National University of Singapore (NUS) CG4002 Capstone Project by Group B03.

This is the single monorepo for the complete capstone system:

- Unity AR visualiser and gameplay loop
- AI training and Ultra96 FPGA inference
- Embedded firmware for IMU and UWB sensing
- Communications and coordination services

## Problem Context and Objective

Pickleball has grown rapidly since the COVID-19 pandemic, first surging in the United States and then expanding across Southeast Asia. In Singapore, demand for play spaces has increased sharply, indicating a clear shift from niche participation to mainstream recreational adoption.

However, dedicated court infrastructure has not expanded at the same pace. Players often rely on shared or improvised venues such as badminton courts and neighborhood spaces. These spaces are typically noise-sensitive, and the repeated high-impact sound of paddle-ball and ball-floor contact can cause significant disturbance in dense Housing and Development Board (HDB) environments, contributing to rising community friction around play.

This project addresses the challenge of sustaining pickleball growth and accessibility in densely populated Singaporean neighborhoods while providing a noise-free, space-efficient training and gameplay experience that is not limited by the availability of dedicated pickleball courts.

If left unresolved, noise disputes may reduce community tolerance for recreational play and constrain the sport's continued growth. The significance of this project lies in balancing the needs of pickleball enthusiasts and residents by reducing dependence on scarce, noise-sensitive physical courts while preserving play accessibility.

The primary objective is to build a comprehensive Augmented Reality (AR) pickleball training and gameplay system that operates reliably in limited spaces. To achieve this, the project integrates:

- Wearable and racket-embedded IMU sensing for real-time swing dynamics capture
- A multi-task neural network for opponent racket-state and shot-type prediction
- Ultra96 deployment using Vitis HLS fixed-point inference, with scaling handled at the ARM side
- End-to-end system integration across sensing, communications, AI inference, and Unity visualisation

The system is evaluated under realistic gameplay conditions for prediction accuracy, end-to-end latency, and FPGA resource and timing feasibility.

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

The repository is organised into major product folders, with supporting repository folders.

| Path | Purpose | Entry point |
|---|---|---|
| `AI/accelerator` | Model training, quantisation, deployment scripts, and Ultra96 runtime tooling | [AI/accelerator/training/train.py](AI/accelerator/training/train.py) |
| `AI/hls` | HLS implementation and Vivado/Vitis projects | [AI/hls/pickleball_model.cpp](AI/hls/pickleball_model.cpp) |
| `Visualiser` | Unity AR client, gameplay, MQTT integration, and game logic | [Visualiser/Assets/Scenes/MainScene.unity](Visualiser/Assets/Scenes/MainScene.unity) |
| `Hardware/imu` | IMU firmware and motion processing | [Hardware/imu/main_code.ino](Hardware/imu/main_code.ino) |
| `Hardware/uwb` | UWB firmware and player position processing | [Hardware/uwb/UWB_sensor.ino](Hardware/uwb/UWB_sensor.ino) |
| `Communications` | Go services for SSH tunnel, coordination, and metrics | [Communications/main.go](Communications/main.go) |

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

1. IMU entry point: [Hardware/imu/main_code.ino](Hardware/imu/main_code.ino).
2. UWB entry point: [Hardware/uwb/UWB_sensor.ino](Hardware/uwb/UWB_sensor.ino).

### Communications Service

From [Communications](Communications), run:

```bash
go run .
```

Primary files:

- [Communications/main.go](Communications/main.go)
- [Communications/system-coordinator.go](Communications/system-coordinator.go)
- [Communications/network-metrics.go](Communications/network-metrics.go)

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

- [Hardware/imu/config.h](Hardware/imu/config.h)
- [Hardware/uwb/config.h](Hardware/uwb/config.h)

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