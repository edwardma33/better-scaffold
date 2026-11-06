# Better Scaffold

Create a Next.js app with PostgreSQL, Drizzle, Better Auth, and the optional integrations you choose in a terminal wizard.

## Install the CLI

The npm package is prepared locally but has not been published. After publication, the intended commands are:

```sh
bunx better-scaffold@latest init
bunx better-scaffold@latest init my-app
```

For a permanent installation:

```sh
bun add --global better-scaffold
better-scaffold init
```

The package includes the Go executable for macOS, Linux, and Windows on x64 and ARM64. Users need Node.js 22 or later and a supported package manager, but do not need Go. The launcher preserves the current directory and passes arguments and terminal input through to the executable.

## Test the package locally

Build a package for your current platform:

```sh
make package
bun add --global ./dist/better-scaffold-0.1.0.tgz
better-scaffold --version
better-scaffold init --dry-run
```

The host-only package is marked private. It is for local testing and cannot be published accidentally. `make test-package` checks the launcher, including argument forwarding and process exit behavior.

## Prepare a release

Set the version in `npm/better-scaffold/package.json`. The package builder injects that version into the Go binary. Build all six platform binaries and the publishable tarball with:

```sh
make package-all
```

The package includes the launcher, binaries, package README, MIT license, and third-party dependency notices. Credentials, generated apps, preview tools, and source templates are not separate package files. The application templates are embedded in the executable. Packaging reads exact dependency versions from each built binary and preserves their upstream license documents in `third-party/`. Missing required notices prevent packaging.

Once this directory is a GitHub repository, the package workflow tests the host package on macOS, Linux, and Windows. Pushing a `v` tag matching the package version runs the release workflow. It creates a draft GitHub release containing the npm tarball and its SHA-256 checksum. It does not publish to npm.

The release workflow requires successful package and generated-app integration checks on the exact tagged commit. The integration workflow builds minimal, Polar, and Stripe apps and checks email authentication in a browser against disposable PostgreSQL. To run it locally, set `TEST_DATABASE_URL` to a disposable database and run `go run ./cmd/integration-check`. It intentionally does not use `DATABASE_URL` from your shell as a fallback.

For the initial source commit, include `.github`, `cmd`, `dev`, `internal`, `npm`, `scripts`, `go.mod`, `go.sum`, `Makefile`, `README.md`, `LICENSE`, and `.gitignore`. Generated apps, preview files, package output, and credentials stay outside the source commit.

Before the first publication, confirm ownership of the npm package name. After reviewing the all-platform tarball, publish it explicitly with `npm publish ./dist/better-scaffold-0.1.0.tgz --access public`. Future versions use the corresponding versioned filename. No registry credentials are stored in this project.

## License

Better Scaffold and its original templates are licensed under [MIT](LICENSE). Generated apps include the template license notice in `BETTER-SCAFFOLD-LICENSE`. Users choose the license for their own application code and preserve applicable notices for copied code. Third-party dependencies and components retain their own licenses.

## Run the wizard

Install Go 1.26 or later and Node.js 22 or later. Install Bun, pnpm, or npm for the generated app. Bun is the default.

```sh
go mod tidy
go run ./cmd/better-scaffold init
```

You can also pass a destination directory or build a binary.

```sh
go run ./cmd/better-scaffold ./my-app
go build -o bin/better-scaffold ./cmd/better-scaffold
```

Use `go run ./cmd/better-scaffold --dry-run` to review the wizard without creating an app.

The wizard collects the stack, an optional shadcn preset, and optional credentials. Review the selections before generation. An existing destination must be empty.

## Choose integrations

Every app includes Next.js App Router, TypeScript, Tailwind, Geist fonts, shadcn, PostgreSQL, Drizzle, and Better Auth email/password forms. The dashboard validates the session on the server.

Choose Google and GitHub sign-in independently. Each selected provider gets a button on the authentication forms.

Optional integrations include Polar or Stripe through Better Auth, the Vercel AI SDK with OpenAI or Anthropic, Motion, Elysia with Eden, TanStack Query, and Zod application validation. Payments include configuration and webhook handling rather than billing screens.

## Finish app setup

Credentials are stored in the generated, gitignored `.env`. The generated `.env.example` contains placeholders. Skipping credentials leaves the relevant setup steps in the generated README.

The generator creates the authentication schema and SQL migration. Providing `DATABASE_URL` also applies the migration to that database. The final review states whether this will happen. Use a database intended for the new app.

OAuth applications, payment products, and webhook endpoints still require setup in the provider's dashboard. Follow the generated README for callback URLs and commands. Missing optional credentials leave those integrations inactive.

If a command fails, the generator keeps the partial project and reports the failed step. Correct the problem and run the recovery command. Generation does not overwrite a nonempty project.

## Verify changes

Use these commands from the repository root while developing the scaffolder.

```sh
make preview-tui
make preview-app
make verify
```

`preview-tui` opens the wizard with saved sample selections and skips generation. `preview-app` generates a reusable app in `.preview/default`, starts Next.js, and watches the generator sources. Open the URL printed by Next.js. Edit a template, save it, and inspect the updated page in your browser. Press Ctrl+C to stop the preview.

Choose a different saved configuration with `PROFILE`.

```sh
make preview-app PROFILE=dev/presets/polar.json
make preview-tui PROFILE=dev/presets/stripe.json
```

The JSON profiles contain selections only. Put credentials in the preview app's `.env`. Refresh preserves that file and installed dependencies. Preview files are disposable, so make lasting changes in the scaffolder templates. The preview never applies database migrations automatically. To test sign-in, configure a development database and run the preview app's `db:migrate` script yourself.

The watcher recompiles the embedded Go templates before refreshing the app. Dependency changes trigger installation. A failed refresh reports its error and leaves the watcher running for the next edit.

If you change the package manager or shadcn preset, save the selections under a new profile filename to create a separate preview. For a custom port, pass `--port` to `cmd/preview` and update `BETTER_AUTH_URL` in that preview's `.env`.

`make verify` runs Go tests, static checks, and a CLI build. It then generates fresh minimal, Polar, and Stripe apps and checks TypeScript and production builds. These checks require registry access, and production builds fetch the Geist fonts. Failed verification retains its temporary app and prints its path. It does not reuse preview credentials or apply database migrations.

For checks without JavaScript downloads, run:

```sh
go run ./cmd/verify --go-only
```

To generate or refresh a preview without starting the server, run:

```sh
go run ./cmd/preview --once
```

```sh
go test ./...
go vet ./...
go build ./cmd/better-scaffold
```

Verify generated apps with their package manager's typecheck and build scripts. Authentication and migration verification require a disposable PostgreSQL database. OAuth and payment sandbox checks require provider credentials.
