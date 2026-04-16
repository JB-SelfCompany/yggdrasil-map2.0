<div align="center">

# 🗺️ YggMap

Интерактивный визуализатор топологии сети Yggdrasil

[![License](https://img.shields.io/github/license/JB-SelfCompany/yggdrasil-map2.0)](LICENSE)
![Go](https://img.shields.io/badge/go-1.21+-00ADD8.svg)
![Vue](https://img.shields.io/badge/vue-3.x-4FC08D.svg)
[![Visitors](https://visitor-badge.laobi.icu/badge?page_id=JB-SelfCompany.yggdrasil-map2.0)](https://github.com/JB-SelfCompany/yggdrasil-map2.0)

**[English](README.md) | [Русский](#)**

</div>

---

## ✨ Возможности

- **Карта сети в реальном времени** — визуализирует связующее дерево и соединения узлов Yggdrasil
- **Граф с автоматическим размещением** — расстановка узлов через ForceAtlas2
- **Детали узла** — адрес, имя, ОС, версия сборки, задержка, время первого/последнего появления
- **Живые обновления** — обновление графа через WebSocket каждые 10 минут
- **Поиск узлов** — поиск по IPv6-адресу, имени или публичному ключу
- **Тёмная/светлая тема** — переключение между тёмным и светлым режимами интерфейса
- **Легенда цветов узлов** — раскраска по степени связности (серый/синий/голубой/зелёный/янтарный/красный) с отображаемой легендой
- **HTTP-кэширование** — `/api/graph` отдаётся с заголовками ETag и gzip-сжатием для быстрой загрузки
- **REST-предзагрузка** — снимок графа загружается через REST до установки WebSocket-соединения, исключая мигание пустого холста
- **Единый бинарник** — фронтенд Vue 3 встроен в Go-бинарник, отдельный веб-сервер не нужен
- **Yggdrasil v0.5.x** — совместим с современным адресным пространством `200::/7` и CRDT-маршрутизацией

## 📦 Установка

### Сборка из исходников

```bash
git clone https://github.com/JB-SelfCompany/yggdrasil-map2.0
cd yggdrasil-map2.0
bash build.sh
# Бинарник: dist/yggmap-linux-amd64 (или для вашей платформы)
```

Требования: Go 1.21+, Node.js 18+

## 🚀 Использование

```bash
# Запуск с настройками по умолчанию (Linux)
./yggmap

# Открыть браузер по адресу http://127.0.0.1:8080

# Кастомный admin-сокет (Windows/macOS)
./yggmap -socket tcp://127.0.0.1:9001

# Одиночный обход сети и вывод JSON
./yggmap -once

# Кастомный файл конфигурации
./yggmap -config /path/to/config.yaml
```

## ⚙️ Конфигурация

Скопируйте `config.example.yaml` в `~/.yggmap/config.yaml`:

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

## 🔧 Требования

- Yggdrasil v0.5.x с доступным admin-сокетом
- **Linux**: Unix-сокет по пути `/var/run/yggdrasil/yggdrasil.sock` (по умолчанию)
- **Windows/macOS**: TCP-сокет — добавьте `AdminListen: tcp://127.0.0.1:9001` в `yggdrasil.conf`

## 📄 Лицензия

GPL-3.0 — см. [LICENSE](LICENSE)

---

<div align="center">
Made with ❤️ by <a href="https://github.com/JB-SelfCompany">JB-SelfCompany</a>
</div>
