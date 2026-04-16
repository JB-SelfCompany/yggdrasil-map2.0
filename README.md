<div align="center">

# 🗺️ YggMap

Interactive network topology visualizer for the Yggdrasil mesh network

[![License](https://img.shields.io/github/license/JB-SelfCompany/yggdrasil-map2.0)](LICENSE)
![Go](https://img.shields.io/badge/go-1.21+-00ADD8.svg)
![Vue](https://img.shields.io/badge/vue-3.x-4FC08D.svg)
[![Visitors](https://visitor-badge.laobi.icu/badge?page_id=JB-SelfCompany.yggdrasil-map2.0)](https://github.com/JB-SelfCompany/yggdrasil-map2.0)

**[English](#) | [Русский](README.ru.md)**

</div>

---

## ✨ Features

- **Real-time network map** — visualizes the Yggdrasil spanning tree and peer connections
- **Force-directed layout** — automatic graph layout via ForceAtlas2
- **Node details** — address, name, OS, build version, latency, first/last seen
- **Live updates** — WebSocket-powered graph refresh every 10 minutes
- **Node search** — find nodes by IPv6 address, name, or public key
- **Dark/light theme** — toggle between dark and light UI modes
- **Node color legend** — degree-based coloring (gray/blue/cyan/green/amber/red) with on-screen legend
- **HTTP caching** — `/api/graph` served with ETag and gzip compression for fast reloads
- **REST pre-population** — graph snapshot loaded via REST before WebSocket handshake to eliminate blank-canvas flash
- **Single binary** — Vue 3 frontend embedded in Go binary, no separate web server needed
- **Yggdrasil v0.5.x** — compatible with modern `200::/7` address space and CRDT routing

## 📦 Installation

### Build from source

```bash
git clone https://github.com/JB-SelfCompany/yggdrasil-map2.0
cd yggdrasil-map2.0
bash build.sh
# Binary: dist/yggmap-linux-amd64 (or your platform)
```

Requirements: Go 1.21+, Node.js 18+

## 🚀 Usage

```bash
# Start with default settings (Linux)
./yggmap

# Open browser at http://127.0.0.1:8080

# Custom admin socket (Windows/macOS)
./yggmap -socket tcp://127.0.0.1:9001

# Crawl once and print JSON
./yggmap -once

# Custom config file
./yggmap -config /path/to/config.yaml
```

## ⚙️ Configuration

Copy `config.example.yaml` to `~/.yggmap/config.yaml`:

```yaml
admin:
  socket: "unix:///var/run/yggdrasil/yggdrasil.sock"
crawler:
  interval: 10m
  enable_nodeinfo: true
server:
  bind: "127.0.0.1"
  port: 8080
```

## 🔧 Requirements

- Yggdrasil v0.5.x running with admin socket accessible
- **Linux**: Unix socket at `/var/run/yggdrasil/yggdrasil.sock` (default)
- **Windows/macOS**: TCP socket — add `AdminListen: tcp://127.0.0.1:9001` to `yggdrasil.conf`

## 📄 License

GPL-3.0 — see [LICENSE](LICENSE)

---

<div align="center">
Made with ❤️ by <a href="https://github.com/JB-SelfCompany">JB-SelfCompany</a>
</div>
