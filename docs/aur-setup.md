# AUR Automation Setup (Safe)

This guide prepares automatic AUR publishing for `dwellir-bin` from GitHub Actions.

## What Stays Manual

1. Create/verify your AUR account (email verification step in AUR UI).
2. Create the `dwellir-bin` package repository on AUR (first-time package submission).
3. Add your AUR SSH public key in AUR account settings.

After this one-time setup, releases can be automatic via CI.

## 1. Generate a Dedicated AUR SSH Key

Use a dedicated key only for AUR automation (do not reuse personal keys).

```bash
ssh-keygen -t ed25519 -C "dwellir-aur-ci" -f ~/.ssh/dwellir_aur_ci -N ''
```

This creates:
- Private key: `~/.ssh/dwellir_aur_ci`
- Public key: `~/.ssh/dwellir_aur_ci.pub`

## 2. Add Public Key to AUR Account

Copy public key:

```bash
cat ~/.ssh/dwellir_aur_ci.pub
```

In AUR account settings, add that public key under SSH public keys.

## 3. Add Private Key to GitHub Secret (Safely)

Set repo secret directly from file (recommended):

```bash
gh secret set AUR_SSH_PRIVATE_KEY --repo dwellir-public/cli < ~/.ssh/dwellir_aur_ci
```

Optional package name variable (default is `dwellir-bin`):

```bash
gh variable set AUR_PACKAGE_NAME --repo dwellir-public/cli --body "dwellir-bin"
```

## 4. Verify SSH Access Without Leaking Secrets

Use test auth call with explicit key:

```bash
ssh -i ~/.ssh/dwellir_aur_ci -T aur@aur.archlinux.org
```

Expected: authentication success message (AUR does not provide shell access).

## 5. Security Rules (Important)

- Never commit private keys to git.
- Never paste private key in issue comments/PR comments.
- Use only GitHub encrypted secrets for private key storage.
- Keep key scope narrow: one key per repo/workflow.
- Rotate key immediately if exposure is suspected.

## 6. Workflow Behavior

Once configured:
- `.github/workflows/aur-release.yml` triggers on GitHub release publish.
- It renders `PKGBUILD` from release checksums, generates `.SRCINFO`, verifies sources, and pushes updates to AUR.

If secret is missing, workflow exits with a clear no-op message and does not fail release publishing.
