<!-- Header -->
<div align="center">

# 🎬 MediaSaveBot

[![Telegram Bot](https://img.shields.io/badge/Telegram%20Bot-Active-blue?style=flat-square&logo=telegram)](https://t.me/xbotsupports)
[![Go Version](https://img.shields.io/badge/Go-1.24.0-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/Beesonn/MediaSaveBot?style=flat-square&logo=github)](https://github.com/Beesonn/MediaSaveBot)
[![GitHub Forks](https://img.shields.io/github/forks/Beesonn/MediaSaveBot?style=flat-square&logo=github)](https://github.com/Beesonn/MediaSaveBot)

**A powerful Telegram bot for downloading media from Instagram, Pinterest, Spotify, and YouTube. Built with Go and gotgbot.**

[🚀 Quick Start](#-quick-start) • [✨ Features](#-features) • [📦 Installation](#-installation) • [💬 Support](#-support) • [⭐ Star Us](#-show-your-support)

</div>

---

## ✨ Features

### 📥 **Multi-Platform Media Downloads**
- 📸 **Instagram** - Photos, videos, carousels, reels
- 📌 **Pinterest** - Pins, boards, high-quality images
- 🎵 **Spotify** - Tracks, albums, playlists with metadata
- 🎬 **YouTube** - Videos, shorts, audio extraction

### 🤖 **Smart Bot Features**
- ⚡ **One-Click Download** - Just send a link
- 📊 **User Statistics** - Track bot usage (Admin only)
- 📢 **Broadcast Messages** - Send to all users with progress tracking
- 🛑 **Stop Broadcast** - Cancel active broadcasts instantly
- 💾 **User Database** - MongoDB integration for user tracking

### 🔐 **Advanced Functionality**
- ✅ **Admin Authentication** - Secure admin commands
- 🔄 **Retry Mechanism** - Handle Telegram flood limits
- 🚫 **Duplicate Prevention** - Prevent concurrent requests per user
- ⚙️ **Batch Processing** - Download multiple tracks at once
- 📈 **Progress Indicators** - Real-time download status

---

## 🚀 Quick Start

### Prerequisites
- **Go** 1.24.0 or higher
- **MongoDB** 4.4+ (optional, for user tracking)
- **Telegram Bot Token** (create via [@BotFather](https://t.me/botfather))

### Installation

1. **Clone the Repository**
   ```bash
   git clone https://github.com/Beesonn/MediaSaveBot.git
   cd MediaSaveBot
   ```

2. **Install Dependencies**
   ```bash
   go mod download
   go mod tidy
   ```

3. **Set Environment Variables**
   ```bash
   # Required
   export TOKEN="your-telegram-bot-token"
   
   # Optional (for user tracking)
   export MONGODB_URI="mongodb://localhost:27017"
   
   # Optional (for admin commands - space-separated user IDs)
   export ADMIN="123456789 987654321"
   ```

4. **Run the Bot**
   ```bash
   go run main.go
   ```

---

## 📋 Commands

| Command | Description | Access | Example |
|---------|-------------|--------|---------|
| `/start` | Initialize bot & show welcome message | Public | Send `/start` to the bot |
| `/stats` | View total active users | Admin | `/stats` |
| `/broadcast` | Send message to all users | Admin | Reply to a message with `/broadcast` |

---

## 🎯 How to Use

### **Download from Instagram**
1. Send any Instagram link (instagram.com or instagr.am)
2. Bot automatically detects and downloads
3. Receive media with captions

```
Example: https://www.instagram.com/p/ABC123XYZ/
```

### **Download from Pinterest**
1. Send Pinterest link (pinterest.com or pin.it)
2. Instant high-quality image download
3. Multiple images in one go

```
Example: https://www.pinterest.com/pin/123456789/
```

### **Download Spotify Music**
1. Send Spotify track, album, or playlist link
2. Bot shows found tracks count
3. Downloads with artist & title metadata
4. Handles playlists with batch processing

```
Example: https://open.spotify.com/track/ABC123XYZ
```

### **Admin: Broadcast Messages**
1. Reply to any message with `/broadcast`
2. Message sends to all active users
3. Real-time progress shows
4. Use 🛑 button to stop

---

## 📂 Project Structure

```
MediaSaveBot/
├── 📄 main.go                    # Entry point & bot dispatcher
├── 📄 go.mod                     # Go dependencies
├── 📄 go.sum                     # Dependency checksums
├── 📄 LICENSE                    # MIT License
├── 📄 README.md                  # This file
│
├── 🤖 bot/
│   ├── commands.go              # /start, /stats, /broadcast handlers
│   └── admin.go                 # Admin functions, broadcast logic
│
├── 💾 database/
│   └── db.go                    # MongoDB user operations
│
└── 🛠️ utils/
    ├── instagram.go             # Instagram downloader
    ├── pinterest.go             # Pinterest downloader
    └── spotify.go               # Spotify downloader
```

---

## 🔧 Configuration

### Environment Variables

```bash
# ⚠️ REQUIRED
TOKEN="your-telegram-bot-token"

# Optional but recommended
MONGODB_URI="mongodb://username:password@localhost:27017"
ADMIN="123456789 987654321 111111111"  # Space-separated admin IDs
```

### Setup Steps

1. **Get Telegram Bot Token**
   - Chat with [@BotFather](https://t.me/botfather) on Telegram
   - Create new bot with `/newbot`
   - Copy the token

2. **Setup MongoDB (Optional)**
   ```bash
   # Using Docker
   docker run -d -p 27017:27017 --name mongodb mongo:latest
   
   # Connection string: mongodb://localhost:27017
   ```

3. **Set Admin IDs**
   - Get your Telegram User ID from [@userinfobot](https://t.me/userinfobot)
   - Add to `ADMIN` environment variable

---

## 📊 Supported Platforms

| Platform | Support | Features |
|----------|---------|----------|
| 📸 Instagram | ✅ Full | Photos, Videos, Carousels, Captions |
| 📌 Pinterest | ✅ Full | Images, High Quality, Metadata |
| 🎵 Spotify | ✅ Full | Tracks, Albums, Playlists, Audio |
| 🎬 YouTube | 🔄 Planned | Video & Audio Downloads |

---

## 🏗️ Architecture

### **Handler Flow**
```
User sends link
    ↓
/HandleMessage detects platform (regex)
    ↓
Platform-specific handler (Instagram/Pinterest/Spotify)
    ↓
dlkitgo library processes download
    ↓
Send media back to user
```

### **Admin Operations**
```
Admin sends /broadcast
    ↓
Database fetches all users
    ↓
Loop through users with batch sending
    ↓
Real-time progress updates
    ↓
User can stop with callback button
```

### **Database Schema**
```javascript
// MongoDB - media_save_bot.users
{
  _id: ObjectId(),
  user_id: 123456789,        // Telegram User ID
  name: "John Doe"           // User's first name
}
```

---

## 🔐 Security Features

✅ **Admin Authentication** - Only whitelisted users can use admin commands  
✅ **Flood Wait Handling** - Automatic retry with exponential backoff  
✅ **Rate Limiting** - Prevents duplicate concurrent requests per user  
✅ **Error Handling** - Graceful degradation with user-friendly messages  
✅ **Input Validation** - Regex pattern matching for URLs  

---

## 📦 Dependencies

```go
// Telegram Bot Framework
github.com/PaulSonOfLars/gotgbot/v2 v2.0.0-rc.34

// Media Download Kit
github.com/Beesonn/dlkitgo v1.2.4

// Database Driver
go.mongodb.org/mongo-driver v1.17.9
```

---

## 💻 System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| **Go** | 1.21.0 | 1.24.0+ |
| **RAM** | 128 MB | 512 MB |
| **Disk** | 100 MB | 500 MB |
| **Network** | 512 kbps | 5 Mbps |
| **OS** | Linux/macOS/Windows | Linux (Ubuntu 20.04+) |

---

## 🚀 Deployment

### **Local Development**
```bash
go run main.go
```

### **Docker Deployment**
```dockerfile
FROM golang:1.24-alpine

WORKDIR /app
COPY . .

RUN go mod download
RUN go build -o mediasavebot .

ENV TOKEN=""
ENV MONGODB_URI=""
ENV ADMIN=""

CMD ["./mediasavebot"]
```

### **Build & Run**
```bash
docker build -t mediasavebot .
docker run -e TOKEN="your-token" mediasavebot
```

---

## 🆘 Troubleshooting

### Bot doesn't respond
- ✅ Check if `TOKEN` environment variable is set
- ✅ Verify token is valid from [@BotFather](https://t.me/botfather)
- ✅ Check internet connection

### Download fails
- ✅ Verify the link format is correct
- ✅ Ensure media still exists on the platform
- ✅ Check if bot has sufficient permissions

### MongoDB connection error
- ✅ Verify `MONGODB_URI` format: `mongodb://host:port`
- ✅ Check if MongoDB service is running
- ✅ Database is optional; bot works without it

### Admin commands don't work
- ✅ Verify your User ID is in `ADMIN` variable
- ✅ Make sure ID format is correct (digits only)
- ✅ Restart bot after changing ADMIN IDs

---

## 📞 Support

### **Get Help**
- 💬 **Support Group**: [@XBOTSUPPORTS](https://t.me/XBOTSUPPORTS)
- 📢 **Updates Channel**: [@BeesonsBots](https://t.me/BeesonsBots)
- 🐛 **Bug Reports**: [GitHub Issues](https://github.com/Beesonn/MediaSaveBot/issues)

---

## 🤝 Contributing

We welcome contributions! Here's how:

1. **Fork the repository**
   ```bash
   git clone https://github.com/YOUR-USERNAME/MediaSaveBot.git
   ```

2. **Create feature branch**
   ```bash
   git checkout -b feature/amazing-feature
   ```

3. **Make your changes**
   ```bash
   git add .
   git commit -m "Add amazing feature"
   ```

4. **Push to branch**
   ```bash
   git push origin feature/amazing-feature
   ```

5. **Open Pull Request**
   - Describe your changes clearly
   - Link any related issues
   - Include screenshots/examples

---

## 📝 License

This project is licensed under the **MIT License** - see [LICENSE](LICENSE) file for details.

```
MIT License

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions...
```

---

## 🌟 Show Your Support

If MediaSaveBot helps you, please consider:

- ⭐ **Star** this repository
- 🔗 **Share** with friends
- 🐛 **Report bugs** if you find any
- 💡 **Suggest features** you'd like
- 🤝 **Contribute** to the project

### **Give us a Star! ⭐**
```
https://github.com/Beesonn/MediaSaveBot
```

---

## 📈 Project Statistics

- **Language**: Go 100%
- **License**: MIT
- **Created**: March 2026
- **Last Updated**: April 2026
- **Status**: Active Development ✅

---

## 🔗 Quick Links

| Link | Purpose |
|------|---------|
| [🤖 Telegram Bot](https://t.me/xbotsupports) | Try the bot live |
| [💬 Support Group](https://t.me/XBOTSUPPORTS) | Get help & discuss |
| [📢 Updates Channel](https://t.me/BeesonsBots) | Latest updates |
| [🐛 Issues](https://github.com/Beesonn/MediaSaveBot/issues) | Report problems |
| [🍴 Fork](https://github.com/Beesonn/MediaSaveBot/fork) | Contribute code |

---

## 📧 Contact

- **Developer**: [@Beesonn](https://github.com/Beesonn)
- **Telegram**: [@XBOTSUPPORTS](https://t.me/XBOTSUPPORTS)

---

<div align="center">

### Made with ❤️ by [Beesonn](https://github.com/Beesonn)

**[⬆ back to top](#-mediasavebot)**

</div>
