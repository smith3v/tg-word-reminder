# Telegram Word Pair Reminder Bot

This is a Telegram bot built using Go that allows users to upload word pairs and receive reminders with those pairs.

## Features

- Upload word pairs as a CSV file (`word1,word2` or tab/semicolon separated).
- Clear uploaded word pairs.
- Set the number of pairs to send in reminders.
- Set the frequency of reminders per day.
- Periodic reminders sent to users with random word pairs.

## Prerequisites

- Go 1.25 or newer
- PostgreSQL database
- A Telegram bot token (create one using [BotFather](https://core.telegram.org/bots#botfather))

## Installation

1. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd <repository-directory>
   ```

2. **Create a production `.env` for Docker Compose:**
   ```dotenv
   # .env
   POSTGRES_USER=your_db_user
   POSTGRES_PASSWORD=your_db_password
   POSTGRES_DB=your_db_name
   ```
   Keep these values in sync with the database settings in `config.json`.

3. **Create a configuration file:**
   Copy `config.example.json` to `config.json` and set your Telegram bot token and database credentials to match `.env`.
   ```bash
   cp config.example.json config.json
   ```
   For local development values and `.env.development`, see [Local Development (Docker Compose)](#local-development-docker-compose).

4. **Run the bot with Docker Compose:**
   ```bash
   docker compose up --build
   ```
   Run detached if you prefer:
   ```bash
   docker compose up -d --build
   ```

## Local Development (Docker Compose)

For local development, use Docker Compose to run the bot and PostgreSQL.

1. **Create a development `.env.development` file:**
   ```dotenv
   # .env.development
   POSTGRES_USER=tgwr
   POSTGRES_PASSWORD=tgwr_local_password
   POSTGRES_DB=tgwrdb
   ```
   Keep these values in sync with the database settings in `config.json`.

2. **Update database settings in `config.json`:**
   Set database values to match `.env.development`:
   - host: `db`
   - user: `tgwr`
   - password: `tgwr_local_password`
   - dbname: `tgwrdb`
   - port: `5432`
   - sslmode: `disable`

3. **Start the stack with the development env file:**
   ```bash
   docker compose --env-file .env.development up --build
   ```
   Run detached if you prefer:
   ```bash
   docker compose --env-file .env.development up -d --build
   ```
   Use `--env-file .env.development` with other Compose commands too.

4. **Useful Compose commands:**
   Tail logs for the bot:
   ```bash
   docker compose --env-file .env.development logs -f tg-word-reminder
   ```

5. **Connect to the local database (in-container):**
   ```bash
   docker compose --env-file .env.development exec db psql -U tgwr -d tgwrdb
   ```

6. **Connect to the local database (host):**
   ```bash
   psql -h 127.0.0.1 -U tgwr -d tgwrdb
   ```

## Database Backups (Docker Compose)

The Compose stack includes a `db-backup` service that runs `pg_dump` every hour and keeps four days of plain SQL backups (compressed with gzip) in `./backups`.

Defaults (override via `.env` or `.env.development`):
```dotenv
BACKUP_INTERVAL_SECONDS=3600
BACKUP_RETENTION_DAYS=4
```

Restore a backup:
```bash
gunzip -c backups/<backup-file>.sql.gz | psql -h 127.0.0.1 -U tgwr -d tgwrdb
```

## Testing

Run unit tests locally:
```bash
go test ./...
```

## Usage

You can send a CSV file with word pairs to the bot to upload them. The first two columns are used as `word1` and `word2` (comma, tab, or semicolon separated). Please refer to the example file `example.csv` for the correct format.

- **Commands:**
  - `/start:` initialize your account.
  - `/getpair`: get a random word pair.
  - `/game`: start a quiz session.
  - `/settings`: configure reminders and pair counts.
  - `/export`: download your vocabulary.
  - `/clear`: remove all uploaded word pairs.

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
