# WorkFlow Discord Bot

A Discord bot integrated with PocketBase to track employee time spent in specific voice channels (projects).

## Features

- **Project Tracking**: Automatically tracks time spent in designated "Project" voice channels.
- **Break Exclusions**: Leaves active project logs when moving to a "Break" channel.
- **PocketBase Backend**: Uses PocketBase for data storage, admin UI, and user management.
- **DisGo Library**: High-performance Discord library for Go.

## Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [GCC](https://gcc.gnu.org/) (required for modernc.org/sqlite, or follow CGO-free build instructions if available)

## Setup

1.  **Clone the repository**

    ```bash
    git clone <repo-url>
    cd discord-workflow-bot
    ```

2.  **Environment Variables**
    Create a `.env` file or export the following variable:

    ```bash
    export DISCORD_TOKEN="your_discord_bot_token"
    ```

3.  **Run the application**

    ```bash
    go build -o workflow .
    ./workflow serve
    ```

    This will start PocketBase at `http://127.0.0.1:8090` and the Discord bot.

4.  **Admin UI**
    Navigate to `http://127.0.0.1:8090/_/` to create your admin account.

## Configuration

- **Break Channel**: Update `BreakChannelID` in `handlers.go` with your actual Discord Voice Channel ID for breaks.
- **Projects**: Create records in the `projects` collection in PocketBase.
  - `channel_id`: The Discord Voice Channel ID to track.
  - `name`: Human-readable name (e.g., "Development", "Design").

## Usage

- Add the bot to your server.
- Ensure the bot has permissions to view channels and see voice states.
- When a user joins a voice channel listed in the `projects` collection, a new `work_logs` record is created.
- When they leave or move to the "Break" channel, the record is closed with an `end_time`.

## Development

- **Run in VS Code**: Use the predefined `Run Bot` task.
- **Migrations**: Modifying collections via the UI can generate migration files if `Automigrate` is enabled (it is by default in `main.go`).
