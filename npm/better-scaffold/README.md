# Better Scaffold

Create a Next.js app through an interactive terminal wizard. Every generated app includes TypeScript, Tailwind, shadcn, PostgreSQL, Drizzle, and Better Auth email/password authentication.

## Run the CLI

After this package is published, run:

```sh
bunx better-scaffold@latest init
bunx better-scaffold@latest init my-app
```

To install the command globally after publication, run:

```sh
bun add --global better-scaffold
better-scaffold init
```

The package includes native binaries for macOS, Linux, and Windows on x64 and ARM64. Install Node.js 22 or later and your preferred JavaScript package manager. Go is not required.

## Choose batteries

Add Google and GitHub authentication, Polar or Stripe payments, the Vercel AI SDK, Motion, Elysia with Eden, TanStack Query, and Zod. The wizard accepts a shadcn preset and optional credentials.

Credentials are written to the generated app's gitignored `.env`. A supplied database URL causes the initial migration to run. Skipped credentials leave setup instructions in the generated README.

## Check the installed version

```sh
better-scaffold --version
better-scaffold init --help
```

Local development tarballs contain only the build machine's binary and are marked private. Release tarballs contain all supported binaries. There are no package installation scripts or runtime binary downloads.

## License

Better Scaffold and its original templates are licensed under MIT. See the included LICENSE file. Third-party components retain their own licenses. You choose the license for your own application code and preserve applicable notices for copied template code.
