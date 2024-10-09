# Telegram Word Pair Reminder Bot

This is a Telegram bot built using Go that allows users to upload word pairs and receive reminders with those pairs.

## Features

- Upload word pairs in the format `word1,word2`.
- Clear uploaded word pairs.
- Set the number of pairs to send in reminders.
- Set the frequency of reminders per day.
- Periodic reminders sent to users with random word pairs.

## Prerequisites

- Go 1.23 or newer
- PostgreSQL database
- A Telegram bot token (create one using [BotFather](https://core.telegram.org/bots#botfather))

## Installation

1. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd <repository-directory>
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Create a configuration file:**
   Create a `config.json` file in the root directory of the project with the following structure:
   ```json
   {
       "database": {
           "host": "your-database-host",
           "user": "your-database-user",
           "password": "your-database-password",
           "dbname": "your-database-name",
           "port": your-database-port,
           "sslmode": "require"
       },
       "telegram": {
           "token": "your-telegram-bot-token"
       }
   }
   ```

4. **Run the bot:**
   ```bash
   go run main.go
   ```

## Usage

- **Commands:**
  - `/upload`: Upload word pairs. Format: `word1,word2` (one pair per line).
  - `/clear`: Clear all uploaded word pairs.
  - `/setpairs <number>`: Set the number of pairs to send in reminders.
  - `/setfrequency <number>`: Set the frequency of reminders per day.

## Database Setup

The bot uses a PostgreSQL database. Ensure that the database is set up and accessible based on the configuration provided in `config.json`. The bot will automatically create the necessary tables for storing word pairs and user settings.

## Logging

The bot uses the standard library's `slog` package for logging. Logs will be printed to the console.

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue for any enhancements or bug fixes.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Go](https://golang.org/)
- [Telegram Bot API](https://core.telegram.org/bots/api)
- [GORM](https://gorm.io/)