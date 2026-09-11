# Launcher maintenance and acceptance testing

The launcher is a thin, dependency-free Go executable. It downloads the matching tagged application payload, verifies its release SHA-256 file, uses Node 20–24 when present, or installs the official Node.js `v22.23.2` portable build after verifying `SHASUMS256.txt`. Runtime files stay below `.crm-runtime/node`; source stays in `app`; diagnostic output stays in `logs/setup.log`. A non-secret `.crm-state.json` records the selected project so a cancelled run can resume without silently creating another remote project.

Launcher-owned npm and npx processes use `npm_config_cache` per child process. On Windows this is `%LOCALAPPDATA%\ClientInteractionCRM\npm-cache`; Linux uses the corresponding `os.UserCacheDir()/ClientInteractionCRM/npm-cache`. The launcher never changes global npm configuration or claims ownership of a system Node installation or the user's general npm cache.

On Windows the launcher installs the standalone `Uninstall Client Interaction CRM.exe` under `%LOCALAPPDATA%\ClientInteractionCRM` and writes `install-manifest.json` beside it. The non-secret manifest records only explicit CRM ownership: installation/runtime/cache/AppData/CRM-INFO/shortcut/uninstaller paths and non-secret project identifiers. It never stores passwords, service keys, authentication tokens, OIDC tokens, or login codes. The uninstaller validates the manifest against fixed CRM-owned locations, installation markers and package identity, rejects protected roots and reparse points, previews cleanup, defaults the final confirmation to No, and never deletes cloud projects. Supabase and Vercel CLI logout are separate opt-in choices because they can affect other projects using the same login.

After application preparation, the launcher stores only the last installation directory in the user's OS configuration directory (`%AppData%\ClientInteractionCRM` on Windows and the standard user config directory on Linux). On a later run it offers to resume that prepared installation and reuse its existing branding. No credential, password, token, project secret, or authentication material is stored in this pointer.

On Windows, npm's `.cmd` launchers are never passed to `CreateProcess` or interpolated into a shell command. The launcher maps `npm.cmd` and `npx.cmd` to the adjacent `node.exe` and npm JavaScript entry point, retaining a structured argument array. This supports system Node under `C:\Program Files\nodejs`, portable Node, Unicode/space-containing install paths and project names, and shell-special characters in generated database passwords without `cmd.exe` interpretation.

The launcher pins Supabase CLI `2.117.0` and Vercel CLI `59.15.1`. Their official browser authentication and user-level credential stores are used. Interactive authentication URLs and device/verification codes remain visible in the console but are not persisted to `setup.log`; sensitive captured command output is likewise kept out of the log and known authentication fields are redacted from diagnostics. Each start creates a fresh per-run log, removing unsafe content left by older launcher versions on the next rerun. The application receives only its Supabase URL and browser-safe publishable key. The first CRM user is deliberately created/invited in Supabase Dashboard.

For a new Supabase project, the launcher reads the authenticated user's organizations through JSON CLI output. It selects the only organization automatically or presents a numbered choice. If none exists, it opens the organization Dashboard and refreshes after confirmation. It then presents the current specific Supabase project regions by human-readable location; the list was verified against the [official Supabase region documentation](https://supabase.com/docs/guides/platform/regions) for the `v1.1.0` launcher.

The launcher generates a strong database password, shows it once for the user to save in a password manager, and retains it only in memory. Supabase project creation currently requires `--db-password`, so the value is necessarily present in that child process's argument list for the duration of that one official CLI call; launcher command logging redacts it. Later link and migration commands receive it through the documented `SUPABASE_DB_PASSWORD` environment variable. It is never written to `.crm-state.json`, `.env.local`, application source, or launcher logs.

## Maintainer release procedure

1. Ensure `package.json` and the intended tag use the same numeric version.
2. Run application and launcher validation from a clean checkout.
3. Commit and push the reviewed changes, then create and push a `vX.Y.Z` tag.
4. Download all artifacts from **Build launcher release assets**.
5. Verify every `.sha256` file locally.
6. Create the GitHub Release manually and attach the application ZIP, both launchers, the Windows uninstaller, their checksum files, and `Run-Windows-Setup.cmd`. Do not publish a launcher without its matching payload and uninstaller.

The workflow does not create a tag or GitHub Release.

## Maintainer-only local payload override

This path exists only for acceptance testing before release assets are published. It is not an end-user installation method. Normal launcher runs remain pinned to the immutable matching GitHub Release.

From a clean, committed checkout, create the exact ZIP and checksum produced by the release workflow:

```bash
bash scripts/package-installer-payload.sh
```

Build the Linux launcher (Go 1.22 or newer is required only for this maintainer build):

```bash
mkdir -p installer/dist
cd installer
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=1.1.0" -o dist/client-interaction-crm-setup-linux-amd64 .
cd ..
```

Run acceptance testing with the explicit absolute local path:

```bash
CRM_PAYLOAD_FILE="$(realpath installer/dist/client-interaction-crm-app-v1.1.0.zip)" \
  ./installer/dist/client-interaction-crm-setup-linux-amd64
```

The launcher reads the adjacent `.zip.sha256` by default. To supply the expected checksum explicitly instead:

```bash
CRM_PAYLOAD_FILE="$(realpath installer/dist/client-interaction-crm-app-v1.1.0.zip)" \
CRM_PAYLOAD_SHA256="$(sha256sum installer/dist/client-interaction-crm-app-v1.1.0.zip | cut -d ' ' -f 1)" \
  ./installer/dist/client-interaction-crm-setup-linux-amd64
```

`CRM_PAYLOAD_FILE` must be an absolute path to a regular local file. A relative path, absent checksum, malformed archive, checksum mismatch, or archive traversal attempt stops setup before installation. `CRM_PAYLOAD_SHA256` alone does not activate the override.

## Linux manual acceptance test

Use an x64 Linux account with no relevant CLI login. A preinstalled Node is optional and Git is not required.

1. Download `client-interaction-crm-setup-linux-amd64` and its checksum into a clean directory.
2. Run `sha256sum -c client-interaction-crm-setup-linux-amd64.sha256`.
3. Run `chmod +x client-interaction-crm-setup-linux-amd64`, then `./client-interaction-crm-setup-linux-amd64`.
4. Choose a new path containing spaces, enter branding, and optionally select a logo.
5. Create/sign in to Supabase through the official flow; create a new disposable dedicated project.
6. Create/sign in to Vercel through the official flow and allow deployment.
7. Create/invite the first user in the Dashboard page opened by the launcher.
8. Open the final URL, sign in, verify CRUD, sign out, and verify anonymous CRM access is blocked.
9. Request a password reset and confirm it lands at `<final-url>/update-password`.

## Windows manual acceptance test

Use a Windows x64 non-administrator account with no Git, Node, npm, Supabase CLI, Vercel CLI, Supabase account, or Vercel account.

1. Download `ClientInteractionCRM-Setup-windows-amd64.exe`, `Uninstall Client Interaction CRM.exe`, the application payload, and their checksums to a clean folder.
2. Double-click `Run-Windows-Setup.cmd` (or the private acceptance wrapper before publication). The private wrapper selects the matching local payload and uninstaller, then starts the launcher without requiring PowerShell.
3. An unsigned first release may trigger SmartScreen; inspect the publisher/file and choose to run only if it came from the official release. Do not disable SmartScreen.
4. Choose a user-writable path containing spaces and Unicode, then complete branding.
5. Create Supabase and Vercel accounts in the official browser flows when prompted; create only disposable test resources.
6. Complete the first-user Dashboard step and test the final URL, CRUD, anonymous blocking, and password reset as above.
7. Re-run `Run-Private-Windows-Acceptance.cmd`. Accept the default `Resume this setup? [Y/n]` prompt and verify it automatically selects the prior directory, recognizes the prepared app, offers to reuse branding, and does not silently duplicate a Supabase project.

To remove local files, run `%LOCALAPPDATA%\ClientInteractionCRM\Uninstall Client Interaction CRM.exe`. Review the exact cleanup preview and confirm only if the displayed installation path is correct. Supabase and Vercel CLI logout are off by default. Online projects are never deleted automatically.

## Post-publication download smoke test

After private Windows acceptance succeeds:

1. Change the repository visibility to Public and confirm that the existing `v1.1.0` Release is publicly visible.
2. In a clean Windows folder, download only `ClientInteractionCRM-Setup-windows-amd64.exe`.
3. Confirm that `CRM_PAYLOAD_FILE` is not set, then double-click the executable normally.
4. Confirm that it downloads and checksum-verifies the public GitHub `v1.1.0` application payload.
5. Once the launcher reaches branding/Supabase setup successfully, this download-only smoke test may be cancelled if a complete Windows provisioning acceptance passed immediately beforehand.

Publish an announcement or share the release only after this public-download smoke test passes.
