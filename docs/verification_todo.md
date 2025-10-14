# Verification Checklist

This list describes the manual checks to validate the current OSS gateway build.

1. **Build backend artifacts**
   - From repo root run `make build`.
   - Ensure configuration disables the Exchange for local-only mode (`marketplace_enabled = false` in `settings.ini` or export `TOKLIGENCE_MARKETPLACE_ENABLED=false`).

2. **Launch services**
   - Start the daemon: `./bin/gatewayd` (default bind `:8081`).
   - In a second terminal start the frontend: `cd fe && npm install && npm run dev` (default `http://localhost:5174`).

3. **Root admin login**
   - Visit the SPA, log in with `admin@local` — no email verification required.
   - Confirm the dashboard shows the marketplace unavailable banner when Exchange is disabled.

4. **Bulk import users**
   - Create a CSV (headers `email,role,display_name`).
   - Run `./bin/gateway admin users import --file users.csv --skip-existing`.
   - Refresh the Admin → Users page to confirm accounts are present.

5. **API key issuance & chat proxy**
   - Generate a key for an imported user (`./bin/gateway admin api-keys create --user <id>` or via Admin UI).
   - Call the chat endpoint:
     ```bash
     curl -H "Authorization: Bearer <token>" \
          -H "Content-Type: application/json" \
          -d '{"model":"loopback","messages":[{"role":"user","content":"hi"}]}' \
          http://localhost:8081/v1/chat/completions
     ```
   - Expect a loopback response payload.

6. **Usage ledger inspection**
   - With a valid session cookie or admin user, request `http://localhost:8081/api/v1/usage/logs`.
   - Verify the latest entry includes the `api_key_id` field referencing the key used above.
