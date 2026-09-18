# Configure project credentials

Run setup inside the project directory after `dwellir auth login`.
Use `--device-code` during login when a browser cannot reach the terminal's localhost callback.

```sh
dwellir project setup --chain ethereum --network mainnet --create-key my-project
```

Use `--key-name my-project` to retrieve an existing enabled key.
Key names must be unique. Setup rejects ambiguous endpoints; specify `--node-type` when needed.
Use `dwellir endpoints list` to discover chain and network names.

Setup writes `.env` with mode 0600 and adds an ignore rule.
Use `--env-file .env.local` for frameworks that expect that filename.
Tracked files and unreadable Git indexes are rejected before creating a key.
Existing Dwellir values require `--replace`.
Unrelated single-line settings are preserved. Multiline environment values require manual configuration.

The file contains `DWELLIR_API_KEY`, `DWELLIR_RPC_URL`, and `DWELLIR_WSS_URL`.
Unsupported transports receive empty values, preventing stale URLs after replacement.
Command output reports the filename and configured transports without credentials.
Never print the environment file or commit it.

`--create-key` reuses an enabled key with the same unique name.
Quota flags must match an existing key's quotas or setup stops.
Update quotas separately when that change is intended.
A later filesystem failure can leave a newly created key in the account.
Inspect keys by name before retrying; reuse avoids creating another key.

Device login requires the coordinated Marly and dashboard releases.
The existing local callback remains available with `dwellir auth login`.
