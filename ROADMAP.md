# Roadmap

## First release: authenticated profile dashboard

Running `gh profile` will open an interactive full-screen dashboard for the active GitHub CLI account. The layout will follow a GitHub profile's information hierarchy: identity and links, pinned repositories, then a contribution heatmap.

### 1. Authenticated profile data

- [x] Reuse `go-gh` authentication and host configuration without separate token setup.
- [x] Fetch display name, username, bio, company, location, follower/following counts, profile URL, website, and social links.
- [x] Fetch the published profile email through authenticated REST `GET /users/{login}`. Do not query private email lists or require broader scopes just for GraphQL's email field.
- [x] Fetch pinned repositories in API order: owner/name, visibility, description, URL, primary language, stars, and forks.
- [x] Fetch the rolling-year contribution calendar, including dates, counts, relative intensity levels, and total contributions.
- [x] Handle pagination, missing optional fields, empty sections, authentication/network failures, and cancellation.

**Acceptance:** Data comes from the active account's API responses. Unset optional metadata is omitted; failed requests are not presented as empty sections. No credentials or private email lists are displayed.

### 2. Profile layout and pinned cards

- [x] Build the TUI with Bubble Tea v2, Bubbles viewport, and Lip Gloss v2.
- [x] Show profile details in a sidebar on wide terminals and above repository cards on narrower terminals.
- [x] Use two repository-card columns when space allows and one column otherwise.
- [x] Wrap descriptions and visible URLs using terminal-cell widths, including Unicode and long unbroken text.
- [x] Sanitize remote text before styling to prevent terminal-control injection.
- [x] Show an explicit empty state when no repositories are pinned; never substitute popular or recent repositories.

**Acceptance:** Names, counts, links, and repository details remain readable without horizontal overflow at wide and narrow supported terminal sizes.

### 3. Contribution heatmap

- [x] Draw chronological week columns with Sunday-to-Saturday rows, month labels, date ranges, and an intensity legend.
- [x] Map GitHub's five contribution levels to neutral/green cells rather than inventing count thresholds.
- [x] Preserve partial edge weeks and distinguish missing dates from real zero-contribution days.
- [x] Wrap whole week columns into successive calendar blocks on narrow terminals, preserving the entire returned period.
- [x] Provide meaningful intensity symbols when `NO_COLOR` is set.
- [x] Explain that totals depend on GitHub visibility and granted access; authenticated aggregate counts may include private contributions.

**Acceptance:** Every returned contribution week is reachable, dates align with weekdays, and the calendar remains meaningful with or without color.

### 4. Terminal interaction and lifecycle

- [x] Start with a loading view and fetch asynchronously.
- [x] Support arrows and `j`/`k`, PageUp/PageDown, Home/End, and `q`/Ctrl-C.
- [x] Keep a one-line controls footer and scroll the profile body vertically.
- [x] Reflow on resize without refetching, preserving and clamping the scroll position.
- [x] Show a bounded resize warning below 30 columns or 10 rows and recover after enlargement.
- [x] Require interactive stdin/stdout and report unsupported terminal execution clearly.
- [x] Cancel pending requests on exit and restore the terminal on success or failure.
- [x] Show actionable load errors and return a nonzero status after a failed load; ordinary user exit succeeds.

**Acceptance:** Users can reach every section, resize the running dashboard, quit during loading, and return to a usable shell.

### 5. Verification and release readiness

- [x] Add deterministic behavior tests for API decoding, pagination, public-email handling, empty/error states, and cancellation.
- [x] Cover Unicode/control-character safety, responsive layout, leap days, partial weeks, intensity levels, and scroll/resize transitions.
- [x] Run `go test ./...` and build the `gh-profile` executable.
- [x] Install the local extension with `gh extension install .` and exercise `gh profile` in a real terminal.
- [x] Verify wide, narrow, undersized, and resized layouts; inspect colors, scrolling, cancellation, and terminal restoration.
- [x] Compare live dashboard fields with fresh authenticated API responses. If connectivity blocks a complete live run, report that separately from fixture-based terminal verification.
- [x] Document installation, authentication, controls, terminal requirements, and contribution-visibility semantics.
- [ ] Use the existing tagged-release workflow to distribute precompiled extension binaries.

**Acceptance:** Automated behavior checks pass, the actual TUI has been exercised, and verification distinguishes live-account evidence from fixture evidence.

## First-release boundaries

The first release targets the active authenticated account and its rolling-year contribution calendar. Avatar graphics, profile README rendering, the activity timeline, arbitrary-user lookup, year selection, browser-opening controls, caching, polling, and automatic retries are not included in this roadmap.
