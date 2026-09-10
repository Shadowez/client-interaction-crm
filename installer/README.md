# Launcher maintenance and acceptance testing

The launcher is a thin, dependency-free Go executable. It downloads the matching tagged application payload, verifies its release SHA-256 file, uses Node 20–24 when present, or installs the official Node.js `v22.23.2` portable build after verifying `SHASUMS256.txt`. Runtime files stay below `.crm-runtime/node`; source stays in `app`; diagnostic output stays in `logs/setup.log`. A non-secret `.crm-state.json` records the selected project so a cancelled run can resume without silently creating another remote project.

The launcher pins Supabase CLI `2.117.0` and Vercel CLI `59.15.1`. Their official browser authentication and user-level credential stores are used. The application receives only its Supabase URL and browser-safe publishable key. The first CRM user is deliberately created/invited in Supabase Dashboard.

For a new Supabase project, the launcher reads the authenticated user's organizations through JSON CLI output. It selects the only organization automatically or presents a numbered choice. If none exists, it opens the organization Dashboard and refreshes after confirmation. It then presents the current specific Supabase project regions by human-readable location; the list was verified against the [official Supabase region documentation](https://supabase.com/docs/guides/platform/regions) for the `v1.1.0` launcher.

The launcher generates a strong database password, shows it once for the user to save in a password manager, and retains it only in memory. Supabase project creation currently requires `--db-password`, so the value is necessarily present in that child process's argument list for the duration of that one official CLI call; launcher command logging redacts it. Later link and migration commands receive it through the documented `SUPABASE_DB_PASSWORD` environment variable. It is never written to `.crm-state.json`, `.env.local`, application source, or launcher logs.

## Maintainer release procedure

1. Ensure `package.json` and the intended tag use the same numeric version.
2. Run application and launcher validation from a clean checkout.
3. Commit and push the reviewed changes, then create and push a `vX.Y.Z` tag.
4. Download all artifacts from **Build launcher release assets**.
5. Verify every `.sha256` file locally.
6. Create the GitHub Release manually and attach the application ZIP, its checksum, both launchers, and both launcher checksums. Do not publish a launcher without its matching payload.

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

1. Download `ClientInteractionCRM-Setup-windows-amd64.exe` and its checksum to a clean folder.
2. In PowerShell, compare `(Get-FileHash .\ClientInteractionCRM-Setup-windows-amd64.exe -Algorithm SHA256).Hash` with the checksum file.
3. Double-click the executable. An unsigned first release may trigger SmartScreen; inspect the publisher/file and choose to run only if it came from the official release. Do not disable SmartScreen.
4. Choose a user-writable path containing spaces and Unicode, then complete branding.
5. Create Supabase and Vercel accounts in the official browser flows when prompted; create only disposable test resources.
6. Complete the first-user Dashboard step and test the final URL, CRUD, anonymous blocking, and password reset as above.
7. Re-run the launcher against the same local directory to verify safe recovery after a simulated cancellation.

To remove local setup files, close the launcher and delete the chosen `ClientInteractionCRM` directory. This removes source, portable Node, and logs. It never deletes Supabase or Vercel projects; remove those explicitly in their dashboards if desired.
