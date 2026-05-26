# MediaSaveBot 🎵

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Telegram](https://img.shields.io/badge/Telegram-Bot-2CA5E0?style=flat&logo=telegram)](https://t.me/GeminiAdvbot)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**A powerful Telegram bot for downloading media from Spotify, YouTube, Instagram, and Pinterest.**

## ✨ Features

- 🎵 **Spotify** – Download songs, playlists, and albums
- 🎬 **YouTube** – Download videos & audio (MP4/MP3)
- 📸 **Instagram** – Download photos & videos from posts
- 📌 **Pinterest** – Download images & videos from pins
- 🔍 **Inline Mode** – Search and download from any chat
- 📊 **Playlist Support** – Pagination & batch download
- ⭐ **Donations** – Support with Telegram Stars
- 💻 **100% Open Source** – Free to use and modify

## 🚀 Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/start` | Show bot information | `/start` |
| `/song` | Search and download a song | `/song never gonna give you up` |
| `/donate` | Support the bot with Stars | `/donate 100` |

## 📥 Supported Links

- **Spotify** – `open.spotify.com/track`, `playlist`, `album`
- **YouTube** – `youtu.be`, `youtube.com/watch`, `playlist`, `shorts`
- **Instagram** – `instagram.com/p`, `reel`
- **Pinterest** – `pinterest.com/pin`

## 🛠️ Installation

**Prerequisites**
- Go 1.21 or higher
- Telegram Bot Token (from [@BotFather](https://t.me/BotFather))

**1. Clone the repository**
```

git clone https://github.com/Beesonn/MediaSaveBot.git
cd MediaSaveBot

```

**2. Install dependencies**
```

go mod tidy

```

**3. Set your bot token**
```

export TOKEN="your_telegram_bot_token"

```

**4. Run the bot**
```

go build .
./MediaSaveBot

```

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `TOKEN` | Yes | Bot token from @BotFather |
| `ADMIN` | No | Admin user IDs (space-separated) |
| `MONGODB_URI` | No | MongoDB for user statistics |

## 💝 Support

- **Support Group**: [@XBOTSUPPORTS](https://t.me/XBOTSUPPORTS)
- **Update Channel**: [@BeesonsBots](https://t.me/BeesonsBots)
- **GitHub**: [Star & Fork](https://github.com/Beesonn/MediaSaveBot)

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📄 License

MIT License