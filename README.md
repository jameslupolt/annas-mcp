# Anna's Archive MCP Server (and CLI Tool)

[An MCP server](https://modelcontextprotocol.io/introduction) and CLI tool for searching and downloading documents (books and articles) from [Anna's Archive](https://annas-archive.gl), with optional automatic selection of a mirror reported as healthy by [SLUM](https://open-slum.org/).

This version includes account sign-in for book searches, article searches, and
DOI lookup. It builds on [iosifache/annas-mcp](https://github.com/iosifache/annas-mcp).
The original anonymous requests can return a browser check even with a browser
User-Agent. When `ANNAS_SECRET_KEY` is configured, the server signs in, reuses the
session in memory, and refreshes it once if the site challenges a request.
Blocked requests now return errors instead of misleading empty search results.
No browser automation or exported cookies are needed for this sign-in path.

> [!NOTE]
> Notwithstanding prevailing public sentiment regarding Anna's Archive, the platform serves as a comprehensive repository for automated retrieval of documents released under permissive licensing frameworks (including Creative Commons publications and public domain materials). This software does not endorse unauthorized acquisition of copyrighted content and should be regarded solely as a utility. Users are urged to respect the intellectual property rights of authors and acknowledge the considerable effort invested in document creation.

> [!WARNING]
> Please refer to [Mirror Selection](#mirror-selection) if any of the links lead to a non-functional Anna's Archive server.

## Requirements

Configure `ANNAS_SECRET_KEY` for signed-in searches. Without a key, anonymous
searches are attempted, but the site may block them with a browser check.

Downloads require:

- [A donation to Anna's Archive](https://annas-archive.gl/donate), which grants JSON API access
- [An API key](https://annas-archive.gl/faq#api)
- An MCP client, such as [Claude Desktop](https://claude.ai/download), if using the project as an MCP server

## Setup

Build this version from source with Go 1.23.4 or newer:

```sh
go build -o annas-mcp ./cmd/annas-mcp
```

On Windows, use `go build -o annas-mcp.exe ./cmd/annas-mcp`. The
[upstream releases](https://github.com/iosifache/annas-mcp/releases) do not include
this version's account-session changes.

Create an `.env` file in the working directory of the process:

```dotenv
ANNAS_SECRET_KEY=your_account_secret_key
ANNAS_DOWNLOAD_PATH=/absolute/path/to/downloads
ANNAS_BASE_URL=annas-archive.gl
ANNAS_AUTO_BASE_URL=false
```

Create the download directory before downloading. Keep the `.env` file private.
For Codex, configure the executable's absolute path and set `cwd` to the folder
containing `.env`:

```toml
[mcp_servers.annas-mcp]
command = "/absolute/path/to/annas-mcp"
args = ["mcp"]
cwd = "/absolute/path/to/annas-mcp-project"
env_vars = ["ANNAS_SECRET_KEY"]
startup_timeout_sec = 30
tool_timeout_sec = 3600
```

Restart the MCP client after changing its executable or configuration.

If you plan to use the tool for its MCP server functionality, you need to integrate it into your MCP client. If you are using Claude Desktop, please consider the following example configuration:

```json
"anna-mcp": {
    "command": "/Users/iosifache/Downloads/annas-mcp",
    "args": ["mcp"],
    "env": {
        "ANNAS_SECRET_KEY": "feedfacecafebeef",
        "ANNAS_DOWNLOAD_PATH": "/Users/iosifache/Downloads",
        "ANNAS_BASE_URL": "annas-archive.gl"
    }
}
```

## Configuration

The tool can be configured with environment variables. These variables can also
be stored in an `.env` file in the process's working directory.

### Account and Download Environment Variables

Search sign-in and downloads use:

- `ANNAS_SECRET_KEY`: The Anna's Archive account secret key, used for account
  sign-in as well as the download API. API downloads require active membership.
- `ANNAS_DOWNLOAD_PATH`: The path where the documents should be downloaded.

### Mirror Selection

Anna's Archive has multiple mirrors, and their availability can change over time. By default, this project uses `ANNAS_BASE_URL`, or `annas-archive.gl` when `ANNAS_BASE_URL` is not set.

Optionally, you can set:

- `ANNAS_BASE_URL`: The Anna mirror to use (defaults to `annas-archive.gl`). When automatic mirror discovery is enabled, this becomes the fallback mirror.
- `ANNAS_AUTO_BASE_URL`: Set to `true` to discover the best available Anna mirror automatically from [SLUM](https://open-slum.org/).

Automatic discovery is opt-in: when `ANNAS_AUTO_BASE_URL=true`, the tool reads the public status page, ranks discovered Anna mirror candidates by recent health and latency, probes them locally, and uses the best reachable mirror. If discovery or probing fails, the tool falls back to `ANNAS_BASE_URL`, then to the built-in default mirror.

The legacy SLUM heartbeat endpoint returned HTTP 404 during the September 2026
verification. Keep `ANNAS_AUTO_BASE_URL=false` and configure a working official
mirror until the discovery implementation is updated.

### Timeouts

HTTP requests default to a 1 hour timeout. For CLI usage, override this with `--timeout`, for example:

```bash
annas-mcp --timeout 1h book-download abc123def456 "my-book.pdf"
```

For MCP usage, tools accept an optional `timeout_seconds` parameter, for example `3600` for 1 hour.

## Demo

### As an MCP Server

<img src="screenshots/claude.png" width="600px"/>

### As a CLI Tool

<img src="screenshots/cli.png" width="400px"/>

## Available Operations

| Operation                                      | MCP Tool           | CLI Command         | Example                                                      |
| ---------------------------------------------- | ------------------ | ------------------- | ------------------------------------------------------------ |
| Search for books by title, author, or topic   | `book_search`      | `book-search`       | `book-search "machine learning python"`                     |
| Download a book by its MD5 hash                | `book_download`    | `book-download`     | `book-download abc123def456 "my-book.pdf"`                  |
| Search for articles by DOI or keywords        | `article_search`   | `article-search`    | `article-search "10.1038/nature12345"` or `article-search "neural networks"` |
| Download an article by its DOI                 | `article_download` | `article-download`  | `article-download "10.1038/nature12345"`                    |

## Validation

```sh
go test ./...
```

Tests cover session reuse and refresh, invalid keys, blocked searches, and
cross-origin redirect rejection, alongside the existing configuration and mirror
tests. Live checks on September 12, 2026 verified book search, article search,
DOI lookup, and downloads with an active member account; downloaded file hashes
matched their Anna's Archive MD5 identifiers. Site behavior can change.

The account-sign-in approach was identified in
[bitesized/annas-archive-api](https://github.com/bitesized/annas-archive-api/blob/main/lib/annas.js)
and independently implemented in Go here. Session cookies remain in memory, and
download URLs containing credentials or signed tokens are omitted from normal logs.
