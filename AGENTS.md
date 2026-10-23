Bootstrapped from [mtlynch's source](https://codeberg.org/mtlynch/go-app-starter/src/branch/master/AGENTS.md)

# Project overview

Wishlist is a web app for an annual adopt-a-family gift drive.

## Roles

- Admins run the drive.
  - Admins log in with a username and password, seeded from the CLI. There is no signup flow.
  - All admins have the same permissions.
- Givers adopt a family and buy gifts for its members.
  - Givers have no accounts.
- Recipients are the families. They never use the app.

## Domain

- Admins collect wishlists from recipients offline and enter them into the app.
- A family groups the recipients in one household (e.g. a mom and her kids).
  - The app shows a family only by an anonymous label such as "Family 12".
- Each family member has their own wishlist.
  - The app shows a member by an anonymous label such as "Mom" or "Child, 7", plus age and gender.
  - Admins transcribe each member's paper intake sheet. The sheet has free-text "Need" and "Wish" sections, plus clothing sizes (shirt, pant, shoe), preferred brands or colors, preferred toiletry scents, an unscented-products checkbox for sensitive skin, preferred gift cards, and additional info (sports teams, fandoms, hobbies).
  - Each line of the "Need" and "Wish" sections becomes its own wishlist item, tagged `need` or `wish`.
- Recipient data stays anonymous. Never store names, addresses, phone numbers, or other identifying information for recipients.
  - Revisit this only after getting feedback from real admins.

## Claims

- A giver claims an entire family, not individual items or members.
- Claims are one-to-one: each family has at most one giver, and each giver claims one family.
- Claiming a family requires the giver's name, email, and phone.
  - Only admins can read giver contact details.
- Claimed families disappear from the public family listing.
- Only admins can release a claim. Givers cannot unclaim a family.
- Browsing and claiming unclaimed families requires no login or access code.
  - Open decision: nothing stops one person from claiming every family with fake emails. Gating (a shared code, admin approval, etc.) is deferred, but must be decided before launch.
- Givers return to their claimed family through a private link that carries a random token.
  - Open decision: the app only shows the link on screen after claiming, so a giver who loses it can't recover it. Emailing the link (provider, automatic vs. admin-triggered sends) still needs to be decided before launch.
  - The database stores only the SHA-256 hash of each token (`internal/auth/token.go`).
  - Never log a token. The request logger redacts paths under `/claims/`.
  - Pages under `/claims/` send `Cache-Control: no-store` and `Referrer-Policy: no-referrer`.

## Lifecycle

- The drive runs once a year. Each season starts by wiping the database, so the schema has no concept of seasons or campaigns.
- Tracking gift drop-off or delivery is out of scope for now.

## Routes

- `GET` routes render HTML pages. Mutations are JSON endpoints that page scripts call with `fetch`, following the login page's pattern.
- `http.CrossOriginProtection` in `NewServer` guards against CSRF by rejecting cross-origin `POST`, `PUT`, and `DELETE` requests from browsers. There are no CSRF tokens. See [Browser support](#browser-support) for what this assumes.
  - Never change state in a `GET` handler, since the protection skips safe methods.
- Public routes (givers, no login):
  - `GET /`: unclaimed families, each shown as "Family N" with its member labels.
  - `GET /families/{id}`: a family's members, sizes, preferences, needs, and wishes. Returns 404 once the family is claimed.
  - `POST /families/{id}/claim`: takes the giver's name, email, and phone; responds with the giver's private link. Returns 409 if the family is already claimed or the email has already claimed a family.
  - `GET /claims/{token}`: the giver's private view of their family.
- Admin routes (session required):
  - `GET /login`, `POST /login`, `POST /logout`.
  - `GET /admin`: every family with its claim status.
  - `GET /admin/families/new`: one form for transcribing a family's intake sheets, with a section per member.
  - `POST /admin/families`: creates a family with all its members and wishlist items in one transaction. Responds with the new family's ID.
  - `GET /admin/families/{id}`: shows a family's members, its claim status, and the giver's contact details if claimed. It hosts the edit link and the release, new-link, and delete actions.
  - `GET /admin/families/{id}/edit`: the new-family form, prefilled with the family's members and wishlists.
  - `PUT /admin/families/{id}`: replaces a family's members and wishlist items in one transaction, with the same body as `POST /admin/families`. Members and items get new IDs. A claim on the family is kept.
  - `DELETE /admin/families/{id}`: deletes an unclaimed family. Returns 409 while the family is claimed, so admins must release the claim first.
  - `DELETE /admin/families/{id}/claim`: releases a claim.
  - `POST /admin/families/{id}/claim/token`: replaces the giver's link token and returns the new link, so admins can re-send it.

## Stack

- Go `net/http` server, Go `html/template` views styled with Tailwind, SQLite (with Litestream replication in production), deployed to Fly.io.

## Layout

- `cmd/server/main.go`: entrypoint; reads configuration from environment variables.
- `cmd/admin/main.go`: CLI that seeds admin accounts (see [Seeding admins](#seeding-admins)).
- `internal/handlers/`: routes (`routes.go`), HTTP handlers, middleware, sessions, templates, and embedded static files.
- `internal/store/`: the `Store` interface and its SQLite implementation.
  - Migrations live in `internal/store/sqlite/migrations/` as numbered `.sql` files.
- `internal/types/`: domain types shared across packages.
- `style/input.css`: Tailwind source and design tokens.
- `dev-scripts/`: build, lint, run, and deploy scripts that the Makefile wraps.

# Development commands

- Run every command inside the Nix dev shell (`make shell`), which provides Go, Node, SQLite, Litestream, shellcheck, sqlfluff, and flyctl.
- Prefer the Makefile targets below over invoking tools directly.

## Configuration

- The server requires these environment variables:
  - `WISHLIST_DB_PATH`: directory that holds `wishlist.sqlite`.
  - `WISHLIST_SESSION_ENV_KEY`: session signing key. Generate one with `make secrets/hmac`.
  - `PORT` (optional): listen port, defaults to `8080`.
- Copy `.env.example` to `.env`, fill it in, and `source .env` before running the server.
- Production also reads `LITESTREAM_BUCKET`, `LITESTREAM_ENDPOINT`, `LITESTREAM_ACCESS_KEY_ID`, and `LITESTREAM_SECRET_ACCESS_KEY`. When `LITESTREAM_BUCKET` is unset, `docker-entrypoint` starts the app without replication.

## Make targets

- `make server/live`: the main dev loop. Runs `modd`, which rebuilds and restarts the server on Go/template changes and recompiles Tailwind CSS on template or `input.css` changes.
- `make server/build`: build the dev binary to `./tmp/server`.
- `make server/run`: build, then run `./tmp/server`.
- `make css`: compile Tailwind CSS (see [Tailwind CSS](#tailwind-css)).
- `make test/go`: run Go unit tests.
- `make lint/go`: run `gofmt -s -w`. This rewrites files in place rather than only reporting problems.
- `make lint/shell`: run shellcheck on every bash script tracked by git.
- `make lint/sql`: run sqlfluff with the SQLite dialect.
- `make lint/frontend`: run `prettier --check` over the repo, including Go HTML templates.
- `make secrets/hmac` / `make secrets/gcm`: generate random keys.
- `make admin/build`: build the admin CLI to `./tmp/admin`.
- `make server/deploy`: run `fly deploy`. Do not deploy unless the user explicitly asks.

## Seeding admins

- `admin create <username>` prompts for the password twice without echoing it, and needs `WISHLIST_DB_PATH`. It runs migrations, so it works on a fresh database.
  - When stdin is not a terminal, it reads the password from the first line of stdin instead, for scripted use.
  - Locally: `make admin/build`, then `./tmp/admin create <username>`.
  - In production, the Docker image ships the CLI as `/app/wishlist-admin`. Run `fly ssh console`, then `/app/wishlist-admin create <username>`.

# Frontend

## Templates

- Views are Go `html/template` files in `internal/handlers/templates/`.
  - `layouts/base.html` renders the document shell and defines the `title` and `script-tags` blocks.
  - `layouts/app.html` renders the authenticated app chrome (main column and bottom nav) and defines the `page-content` block.
  - `pages/*.html` fill in those blocks.
- Prettier with `prettier-plugin-go-template` formats templates. Run `npx prettier --write <file>` after editing them.

## Tailwind CSS

- Tailwind v4 compiles `style/input.css` into `internal/handlers/static/css/style.css`.
- `style.css` is generated but committed, and the Go binary embeds it via `go:embed`. Neither the Dockerfile nor the build scripts run Tailwind.
  - After changing templates or `input.css`, regenerate the CSS and commit it with the change, or production serves stale styles.
  - `make server/live` regenerates it automatically. To regenerate manually, run `make css`.
- Never hand-edit `style.css`.

## Design tokens

- `input.css` drops Tailwind's default palette (`--color-*: initial`). Only the semantic tokens defined in `@theme` exist: `page`, `surface`, `border`, `fg`, `muted`, `subtle`, `accent`, `accent-hover`, `on-accent`, `festive`, `danger`, `danger-surface`.
  - Use them as normal utilities (`bg-page`, `text-muted`, `border-border`).
  - Raw palette classes like `bg-gray-100` silently produce no styles.
- The palette is Christmas-themed: evergreen `accent` for interactive elements, cranberry `festive` for decoration.
  - Use `festive` only for non-interactive highlights such as page titles, the claimed badge, and the top stripe. Interactive red would read as a destructive `danger` action.
  - Check WCAG AA contrast (4.5:1) in both themes when changing token values.
- Dark mode redefines the same tokens under `prefers-color-scheme: dark`. Do not add `dark:` variants for color.
- When a new color role is needed, add a token in both the light and dark blocks rather than reaching for a raw color.

## JavaScript

- The Content Security Policy only allows scripts that carry the per-request nonce. Every `<script>` tag needs `nonce="{{ .CspNonce }}"`.
- Do not use `alert()` or `confirm()`.
- Do not embed Go template conditionals inside JavaScript. Instead, render a data attribute or hidden input in HTML and read it from JS.

## Browser support

- The app assumes a modern evergreen browser for both admins and givers. Page scripts are ES modules (`<script type="module">`) that use `fetch`, so older browsers cannot submit any form.
- CSRF protection depends on browser-sent headers:
  - `http.CrossOriginProtection` reads `Sec-Fetch-Site`, which browsers have sent since roughly Chrome 76, Firefox 90, and Safari 16.4.
  - Without `Sec-Fetch-Site`, it compares the `Origin` header to `Host`.
  - A browser that sends neither header gets no CSRF protection. The middleware lets the request through rather than rejecting it.
- Admin sessions also rely on the `SameSite=Lax` session cookie, which keeps browsers from attaching the cookie to cross-site `POST` requests.
- Givers have no session, so a forged claim request does no more harm than an attacker submitting the claim form directly.
- The `Origin` fallback requires the reverse proxy to forward the browser's `Host` header unchanged. Fly's proxy does. Recheck this if the deployment adds a proxy or custom domain that rewrites `Host`.

# Go

## Defining symbols

- Define variables and structs as close as possible to when they're first used.
  - Exception: Declare all package-level variables and structs at the top of the file above imports.

## Interfaces

- Define interfaces in the client package that consumes them, not in the package that implements them.
  - This follows the Go convention that clients should define interfaces based on what they need.
  - Implementing packages should return concrete types.

## Go modules

- Do not vendor Go modules locally (`go mod vendor`).
- Use Nix `vendorHash` in `flake.nix` for reproducible builds instead.
- When dependencies change, update `vendorHash` by running `nix build` with a fake hash and using the correct hash from the error message.

## Go testing

- When writing new test cases, avoid having `t.Run` have special-case behavior for particular inputs.
  - Instead, use general purpose logic that doesn't assume particular inputs
- Design test table fields to describe the scenario at a high level so the reader can understand the behavior from the table alone.
- Never use `time.Now` in tests. Use a hardcoded fixed time
- Inject `time.Now` as a `func() time.Time` dependency so production code is testable.
  - If a type already injects a clock for one purpose, use that same clock for all time-dependent logic in that type.
- Go tests should be in a separate `_tests` package so they don't test non-exported interfaces
- Test HTTP handlers by sending requests to the relevant routes.
  - Minimize test coupling by avoiding tests that call HTTP handler functions directly
- Use `t.Fatalf` for assertions where the test cannot continue if the assertion fails.
  - Typically, we fail with `t.Fatalf` if a function returned an error when we expected a nil error.
  - If we're just testing that the ouput of a function is what we expect, that's a `t.Errorf` not a `t.Fatalf`.
- Do not write tests that scrape information from `log.Print*` calls.

### if got, want

Use the `if got, want` pattern when writing or editing unit tests. See this snippet as an example:

```go
func TestParseTwitterHandle(t *testing.T) {
  for _, tt := range []struct {
    explanation    string
    input          string
    handleExpected social.TwitterHandle
    errExpected    error
  }{
    {
      "regular handle on its own is valid",
      "jerry",
      social.TwitterHandle("jerry"),
      nil,
    },
    {
      "regular handle in URL is valid",
      "https://twitter.com/jerry",
      social.TwitterHandle("jerry"),
      nil,
    },
    {
      "handle with exactly 15 characters is valid",
      "https://twitter.com/" + strings.Repeat("A", 15),
      social.TwitterHandle(strings.Repeat("A", 15)),
      nil,
    },
    {
      "handle with more than 15 characters is invalid",
      "https://twitter.com/" + strings.Repeat("A", 16),
      social.TwitterHandle(""),
      social.ErrInvalidTwitterHandle,
    },
  } {
    t.Run(fmt.Sprintf("%s [%s]", tt.explanation, tt.input), func(t *testing.T) {
      handle, err := social.ParseTwitterHandle(tt.input)
      if got, want := err, tt.errExpected; got != want {
        t.Fatalf("err=%v, want=%v", got, want)
      }
      if got, want := handle, tt.handleExpected; got != want {
        t.Errorf("handle=%v, want=%v", got, want)
      }
    })
  }
}
```

### Table-driven tests

- The ordering of fields in `TestParseTwitterHandle` is intentional. It goes:
  - Human-facing explanation of what the test is asserting
  - Input(s) to the function under test
  - Output(s) to the function under test

# Callsite legibility

Function calls should be legible from the callsite:

Here's an illegible function call

```go
foo(3, 'foo', 9.2)
```

The call is illegible because it's impossible to infer from the callsite what the parameters mean.

Here are more legible callsites:

```go
angle := 3
rocketName := 'foo'
power := 9.2
foo(angle, rocketName, power)
```

Or change the callee to accept a struct:

```go
	foo(LaunchParams{
		Angle:      3,
		RocketName: "foo",
		Power:      9.2,
	})
```

Dave Cheney's functional options pattern is also a more legible alternative:

```go
func (s Store) ReadReviews(opts ...store.ReadReviewsOption) { ... }

...

s.ReadReviews(
    store.FilterByUsername(user),
    store.FilterByMovieID(id),
)
```

# API design

## Simplicity

- Keep shell scripts and Nix apps as simple as possible.
- Do not add more than one method for achieving the same outcome.
  - If we replace one script or workflow with a better one, do not keep the old one around for legacy purposes or backwards compatibility.
  - Upgrade all references to use the new path.
- Do not add extra flags to support hypothetical scenarios.
- Do not add experimental modes, compatibility shims, aliases, workarounds, or fallback paths.
  - For code we own, prefer a single path that we know works and fix that path when it fails.
- YAGNI.
  - Do not add flags or options until there is a specific need.

## Minimize exported surface area

- Don't export methods just for testing - test through public APIs instead.
- Only export what external packages actually need to use.

## Avoid platform coupling

- Don't pass platform-specific types (e.g., AWS Lambda events) to business logic.
- Create simple structs with only the data needed, making code portable.

## Encapsulate related operations

- Group related operations (e.g., verification + processing) in a single method.
- This simplifies APIs and prevents steps from being accidentally skipped.

## Design for testing

- Consider allowing bypass mechanisms for tests (e.g., empty secret = skip verification).
- Test private methods indirectly through public APIs.
- Structure code so unit tests don't need complex setup (e.g., generating valid signatures).

## Keep interfaces simple

- Group related parameters into structs rather than multiple arguments.
- Return single error types that can represent multiple failure modes.
- One method should have one clear responsibility from the caller's perspective.

## Avoid redundant naming in interface methods

- Method names should not repeat the domain implied by the interface or type name.
- Read the method at its callsite (`s.field.Method()`) to check for redundancy.
- Prefer names like `PasswordResetter.Request(user)` over `PasswordResetter.RequestReset(user)`.

# Assistant guidelines

- Do not ask the user to run a command/test or read an output file that you can read yourself.
  - Aim as much as possible to absorb work from the user.

## Documenting lessons learned

After successfully completing a task where the user had to provide corrections or guidance, consider adding the lessons to `AGENTS.md`. This helps build institutional knowledge and prevents repeating mistakes.

### When to add new guidelines

- The user corrected a misunderstanding about the codebase
- You learned a new pattern or best practice specific to this project
- The user revealed a preference or requirement not previously documented

### How to add guidelines

1. Identify the key principle or pattern learned
2. Determine which section of `AGENTS.md` fits best
3. Add a concise, actionable guideline
4. Keep entries brief but clear for future LLM conversations

### Example

If you learned that methods shouldn't be exported just for testing, add to `AGENTS.md`:

"Don't export methods just for testing - test through public APIs instead."

# Nix flake

- Do not embed scripts in Nix files when the script is longer than 15 lines of code.
  - Keep scripts as standalone files and call them from the flake.
- Avoid string substitution to inject large shell scripts into the flake.
- When communicating executable locations from the flake to shell scripts, prefer updating `PATH` so scripts can execute binaries by name.

# dev-scripts

- Use scripts in the dev-scripts to build, run, and test code where possible.
- Do not depend on Nix in the `dev-scripts`.
  - Nix build targets should leverage scripts from `dev-scripts` rather than reimplementing the same logic in Nix.

# Style conventions

- End comments with trailing punctuation.
- Break code comment lines at 80 characters.
  - Treat tabs as two spaces.
- Never keep dead code for the sake of backwards compatibility.
  - If there are no calls to a function or uses of a type/variable outside of test code, it is dead code and should be deleted.

## Markdown

- Do not hard wrap Markdown text outside code snippets.
- Generally, prefer bulleted lists to tables.

### Headings

- Use sentence casing and not title casing.
- Do not add trailing periods.

# Shell scripts

- Attempt to break bash lines at 80 characters, but don't layer in complexity to fit within 80 characters.
  - e.g., if a long URL forces the line beyond 80 characters, it's better to just have the long line than to break up the URL into parts that are below 80 characters to fit the limit.
- Prefer passing information to shell scripts through command-line arguments rather than environment variables.
- When removing a command-line argument, delete it cleanly.
  - Do not add compatibility checks for the old interface.

# Testing

- After every code change, run `make test/go` and the lint targets relevant to the files you touched (`make lint/go`, `lint/shell`, `lint/sql`, `lint/frontend`) before presenting the solution to the user.
- Do not write to the filesystem in tests.
  - Design production interfaces around file readers or filesystem interfaces rather than the local filesystem.
- When writing tests to verify a bugfix, follow TDD conventions: write the test with the failing test first, verify the test fails, fix the bug, and verify that the test passes

## Tests must be correct by inspection

- Minimize complexity and abstraction in tests, as indirection makes it harder to verify correctness by simple inspection.
- Avoid loops except for standard table-driven tests in Go.
- Don't use helper functions that hide information needed to understand a test's correctness.
  - Helper functions in tests are okay as long as they don't hide critical values.
- If we use table-driven tests, one table per function.

### Example: Bad test

```python
def setUp(self):
  database = MockDatabase()
  database.add_row({
      'username': 'joe123',
      'score': 150.0
    })
  self.account_manager = AccountManager(database)

def test_initial_score(self):
  initial_score = self.account_manager.get_score(username='joe123')
  self.assertEqual(150.0, initial_score)
```

This is an example of a bad test because the reader can't reason from `test_initial_score` why the assertion is correct. They'd have to go outside the test function to see the value of 150.0.

### Example: Good test with helper method

Helper methods in tests are okay as long as they don't hide critical values:

```python
def make_dummy_account(self, username, score):
  return Account(username=username,
                 name='Dummy User',         # <- OK: Buries values but they're
                 email='dummy@example.com', # <-     irrelevant to the test
                 score=score)

def test_increase_score(self):
  account_manager = AccountManager()
  account_manager.add_account(
    make_dummy_account(
      username='joe123',  # <- GOOD: Relevant values stay
      score=150.0))       # <-       in the test

  account_manager.adjust_score(username='joe123',
                               adjustment=25.0)

  self.assertEqual(175.0,
                   account_manager.get_score(username='joe123'))
```

# Documentation

- Use active voice in documentation.
  - Prefer clear `<subject> <verb> <object>` structure.
  - Especially avoid passive voice phrasing that obscures which component performs which action (e.g., "the token **is passed**").

# Git

## Pre-commit checks

- If git pre-commit hooks fail, pause and fix the failures.
- Do not ignore pre-commit failures.
- Never bypass hooks with `--no-verify`.

## Commit message conventions

- Always write commit messages with single-quoted heredocs so bash characters do not interpolate within the message.
- Make the first line describe the effect of the change, not the implementation details.
- Put the most important information first.
- Use the body to explain motivation, user impact, and relevant background.
- Include searchable details such as exact error messages when they aid future debugging.
- Leave out details that are obvious from the diff.
- Do not bury critical maintenance constraints in commit messages.
  - Enforce them in code, tests, or docs.

## Remotes

- Do not attempt to push, pull, or fetch from remotes unless the user instructs you to do so directly.
  - You do not have access to any private git repos, so most attempts will fail.
