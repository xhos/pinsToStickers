# Itanoru - Pinterest to Telegram Sticker Bot

Itanoru is a Telegram bot that synchronizes Pinterest boards with Telegram sticker packs. It automatically downloads images from Pinterest boards, processes them into the correct sticker format, and keeps your Telegram sticker packs in sync with your Pinterest boards.

## Features

- 🎨 **Automatic Sync**: Synchronizes Pinterest boards with Telegram sticker packs
- 📱 **Telegram Integration**: Create and manage sticker packs directly through Telegram
- 🖼️ **Image Processing**: Automatically converts images to WebP format optimized for stickers
- 🔄 **Scheduled Updates**: Hourly synchronization to keep sticker packs current
- 🐳 **Docker Support**: Easy deployment with Docker and docker-compose
- 📊 **Database Tracking**: SQLite database to track packs, stickers, and sync history
- ⚡ **Concurrent Processing**: Efficiently handles multiple packs simultaneously

## Commands

- `/start` - Welcome message and instructions
- `/newpack <pinterest_board_url>` - Create new sticker pack from Pinterest board
- `/listpacks` - Show all your sticker packs  
- `/sync <pack_name>` - Manually sync a specific pack
- `/syncall` - Sync all your packs
- `/packinfo <pack_name>` - Show detailed pack information
- `/deletepack <pack_name>` - Remove pack management
- `/help` - Show help message

## Quick Start

### Prerequisites

- Go 1.21+
- gallery-dl
- Docker (optional)
- Telegram Bot Token from [@BotFather](https://t.me/botfather)

### Local Development

1. **Clone the repository**
   ```bash
   git clone https://github.com/xhos/itanoru.git
   cd itanoru
   ```

2. **Install dependencies**
   ```bash
   go mod tidy
   ```

3. **Install gallery-dl**
   ```bash
   pip install gallery-dl
   ```

4. **Configure environment**
   ```bash
   cp .env.example .env
   # Edit .env with your bot token
   ```

5. **Run the bot**
   ```bash
   go run cmd/bot/main.go
   ```

### Docker Deployment

1. **Create environment file**
   ```bash
   cp .env.example .env
   # Set BOT_TOKEN in .env file
   ```

2. **Deploy with docker-compose**
   ```bash
   docker-compose up -d
   ```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `BOT_TOKEN` | Telegram bot token (required) | - |
| `DB_PATH` | SQLite database path | `./data/bot.db` |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `SYNC_INTERVAL` | Cron expression for sync schedule | `@hourly` |
| `MAX_CONCURRENT_SYNCS` | Max simultaneous syncs | `3` |
| `TEMP_DIR` | Temporary files directory | `./temp` |
| `MAX_STICKERS` | Max stickers per pack | `120` |
| `GALLERY_DL_RATE_LIMIT` | Rate limit for downloads | `1.0-2.0` |

### Pinterest Board URLs

The bot supports Pinterest board URLs in the format:
```
https://pinterest.com/username/board-name/
```

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Telegram      │────▶│   Go Bot         │────▶│   Pinterest     │
│   Users         │◀────│   Application    │◀────│   Boards        │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                               │
                               ▼
                    ┌──────────────────────┐
                    │   SQLite Database    │
                    │   gallery-dl         │
                    │   Image Processor    │
                    │   Docker Container   │
                    └──────────────────────┘
```

## Database Schema

The bot uses SQLite with the following main tables:

- **users** - Telegram user information
- **sticker_packs** - Pinterest board to Telegram pack mappings
- **stickers** - Individual stickers and their sync status
- **sync_logs** - History of synchronization operations

## Limitations

- Maximum 120 stickers per pack (Telegram limit)
- Images are converted to 512x512 WebP format
- Only supports static stickers (no animated)
- Requires Pinterest boards to be publicly accessible

## Development

### Project Structure

```
itanoru/
├── cmd/bot/           # Main application entry point
├── internal/
│   ├── bot/           # Telegram bot handlers
│   ├── config/        # Configuration management
│   ├── database/      # Database models and operations
│   ├── image/         # Image processing utilities
│   ├── pinterest/     # Pinterest integration
│   ├── scheduler/     # Sync scheduling
│   ├── sync/          # Synchronization engine
│   └── telegram/      # Telegram API client
├── pkg/
│   ├── logger/        # Logging utilities
│   └── utils/         # Common utilities
└── config/            # Configuration files
```

### Building

```bash
# Build for current platform
go build -o itanoru-bot cmd/bot/main.go

# Build for linux/amd64 (for Docker)
GOOS=linux GOARCH=amd64 go build -o itanoru-bot cmd/bot/main.go
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [gallery-dl](https://github.com/mikf/gallery-dl) - Pinterest content downloading
- [go-telegram-bot](https://github.com/go-telegram/bot) - Telegram Bot API
- [logrus](https://github.com/sirupsen/logrus) - Structured logging