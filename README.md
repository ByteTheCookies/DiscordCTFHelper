# DiscordCTFHelper

<div align="center">
  <img src="images/logo.png" alt="logo" width="400"/>
</div>

> A powerful and customizable bot to manage and organize CTF competitions on Discord.

## Table of Contents

- [About](#about)
- [Features](#features)
- [Commands](#commands)
- [Installation](#installation)
  - [Prerequisites](#prerequisites)
  - [Steps](#steps)
  - [Docker](#docker)
- [Configuration](#configuration)
- [Usage](#usage)
- [License](#license)
- [Roadmap](#roadmap)
- [Contact](#contact)

## About

**DiscordCTFHelper** is a Go bot built with `disgo` designed to streamline CTF (Capture The Flag) management on Discord servers. The bot integrates with CTFTime and provides essential features for tracking events, managing participants, and sharing updates.

## Features

- **One command to publish a CTF**: `/create` reads the event from CTFtime and sets up a role, a private channel, a post in the feed channel and a Discord scheduled event. If a step fails, everything already created is removed, so it can simply be retried.
- **Opt-in participation**: members react to the feed post to get the CTF role and access to its channel, and remove the reaction to leave.
- **Start and end announcements**: the CTF role is pinged when the event starts. When it ends, the channel moves to the archive category and the role is retired.
- **Team votes**: `/vote` opens a poll on the CTFs of the week, or of the next week near the weekend. The winner is created automatically, and a tie is reported to the team.
- **Challenge threads**: `/chall` opens a discussion thread for a challenge while the CTF is running.
- **Flag tracking**: `/flag` announces a solve to the team, credits a teammate, counts it, and locks the thread if used in one.
- **Shared credentials**: `/creds` stores the team's login for a CTF and shows it only to its participants.
- **Results**: `/report` shows place and score from CTFtime, along with the solve count.
- **Upcoming events**: `/next-ctfs` lists the next CTFs on CTFtime.
- **Welcome reaction**: new members are greeted with a reaction, using a custom emoji if configured.
- **Automatic command sync**: slash commands are published at each start, for every server the bot is in or joins later.

## Commands

| Command | Description | Who can use it | Where |
|---|---|---|---|
| `/init` | Configure the bot for the server | Administrators | Any channel |
| `/create` | Publish a CTF from its CTFtime ID | Managers | Any channel |
| `/remove` | Remove a CTF and everything it owns | Managers | Any channel |
| `/vote` | Open a poll on the next CTF | Managers | Any channel |
| `/delete-flag` | Take back a solve from the count | Managers | CTF channel |
| `/next-ctfs` | List the next CTFs on CTFtime | Everyone | Any channel |
| `/flag` | Register a flag | Anyone in the CTF channel | CTF channel or its threads |
| `/chall` | Open a challenge thread | Participants, while the CTF is active | CTF channel |
| `/report` | Show the results of the CTF | Participants | CTF channel |
| `/creds` | Show or set the CTF credentials | Participants | CTF channel |
| `/delete-creds` | Delete the CTF credentials | Participants | CTF channel |
| `/help`, `/guide`, `/rules` | Commands, how to play and rules | Everyone | Anywhere |
| `/ping`, `/version` | Latency and build information | Everyone | Anywhere |

Managers hold the role chosen with `/init`, and participants hold the role of the CTF. Being a manager doesn't make someone a participant.

## Installation

### Prerequisites

- Go 1.27.3 or later
- A C compiler such as `gcc`, needed by the SQLite driver
- A Discord bot token (obtainable via the [Discord Developer Portal](https://discord.com/developers/applications))

### Steps

1. Clone this repository:

   ```bash
   git clone https://github.com/ByteTheCookies/DiscordCTFHelper.git
   cd DiscordCTFHelper
   ```

2. Install dependencies:

   ```bash
   go mod download
   ```

3. Set up your `.env` file:
   Create a `.env` file in the root directory and include the following variables:

   ```env
   TOKEN=<discord_token>
   LOG_LEVEL=info
   LOG_FORMAT=text
   LOG_SOURCE=false
   ```

4. Create the directory for the database:

   ```bash
   mkdir -p data
   ```

5. Run the bot:

   ```bash
   go run ./cmd/ctfhelper
   ```

   Slash commands are published automatically at each start, globally. Use a separate bot token for development, as running with the production one publishes your local commands to every server.

### Docker

```bash
mkdir -p data
sudo chown -R 1000:1000 data
docker compose up --build
```

The container runs as UID 1000, which needs write access to the directory holding the database.

## Configuration

| Variable | Required | Description |
|---|---|---|
| `TOKEN` | Yes | Discord bot token |
| `LOG_LEVEL` | Yes | `debug`, `info`, `warn` or `error` |
| `LOG_FORMAT` | No | `text` or `json`, `text` by default |
| `LOG_SOURCE` | No | `true` to log the source location |
| `DB_PATH` | No | Database path, `data/ctfs.db` by default |
| `GREET_EMOJI_ID` | No | Custom emoji to greet new members with, 🍪 by default |

## Usage

1. Invite the bot to your server using an OAuth2 link from the [Discord Developer Portal](https://discord.com/developers/applications). It needs permission to manage channels, roles, events, threads and messages, view message history, send messages and polls, create public threads and add reactions. No privileged intents are required.
2. As an administrator, run `/init` to choose the categories for active and archived CTFs, the manager role, the feed channel, your CTFtime team ID and the team role.
3. As a manager, publish a CTF with `/create`, or let the team choose with `/vote`.
4. Members react to the post in the feed channel to join.

## License

This project is licensed under the Apache License. See the [LICENSE](LICENSE.md) file for details.

## Roadmap

- [x] Add final scraping for team stats
- [x] Add overall report generation
- [x] New handling of channel one channel only read width the banner show
- [x] Flag command
- [x] Reaction cookie and fire too each flag
- [x] Remove command
- [x] Creds command (command for store and show the creds of a ctf)
- [x] Send link to ctf message when channel is created
- [x] `/chall` command to add challenges
- [x] Migration to discordgo or disgo (not urgent)
- [x] `/vote` to vote next ctf

## Contact

For support or questions, feel free to open an issue or contact us directly
