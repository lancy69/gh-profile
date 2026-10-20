# gh-profile

![Vibe Coding Badge](./assets/badge-vibe-coding.svg)

An interactive, full-screen GitHub profile dashboard for the active `gh` account and host.

## Install and run

Requires [GitHub CLI](https://cli.github.com/) and an authenticated account:

```sh
gh auth login
gh extension install lancy69/gh-profile
gh profile
```

For a local checkout (Go version specified in `go.mod`):

```sh
go build -o gh-profile .
gh extension install .
gh profile
```

`gh profile --help` prints usage without opening the dashboard or calling the API. No username or other positional arguments are accepted. Input and output must both be terminals; redirected output and `TERM=dumb` are rejected.

## Dashboard

- Display name, username, bio, follower/following counts, company, location, and published links.
- The single public profile email from REST `GET /users/{login}`, omitted when unset. Private account email addresses are never requested; no additional email scope is required.
- All pinned repositories returned to your authenticated account, in GitHub's order, with visibility, description, URL, primary language, stars, and forks.
- GitHub's rolling-year contribution calendar, preserving its returned dates, weeks, total, and relative contribution levels. Counts reflect your account settings and API access, including any visible private contribution aggregates; they can differ from an anonymous browser view. No repository-level contribution details are fetched.

At 144 columns and wider, profile details form a sidebar. Narrower terminals stack sections; the calendar splits into vertically scrollable blocks without dropping weeks. The minimum usable size is 30×10. Resize freely; data is retained without refetching.

| Keys | Action |
| --- | --- |
| `↑` / `↓`, `k` / `j` | Scroll |
| `PgUp` / `PgDn` | Page |
| `Home` / `End` | Top / bottom |
| `q`, `Ctrl-C` | Quit; cancel any in-flight load |

Set `NO_COLOR=1` for an unstyled dashboard whose calendar uses `.`, `1`, `2`, `3`, and `4` instead of green intensities. Unknown API contribution levels appear as `?`. Links remain visible, wrapped text; the extension does not open browsers or emit OSC hyperlinks. Remote text is sanitized before rendering.

API failures are displayed in the dashboard and return a nonzero exit status after quitting. No partial profile is presented as a completed load. For authentication failures, use `gh auth status` and `gh auth login`; transport and rate-limit failures retain their actual causes. Normal quit, including during loading, exits successfully. The extension uses `gh` host/token configuration, disables caching and environment-triggered API logging, and does not change authentication or request expanded scopes.

## Development

```sh
go test ./...
go build -o gh-profile .
```

Tests use real go-gh clients with deterministic in-memory HTTP transports and network-free rendering/lifecycle scenarios.
