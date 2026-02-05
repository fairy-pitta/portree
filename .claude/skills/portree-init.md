# portree-init

Set up portree in a project for multi-branch development with automatic port allocation.

## When to Use

- User wants to set up portree in their project
- User asks about configuring `.portree.toml`
- User wants to run multiple branches simultaneously

## Instructions

1. **Check if portree is installed:**
   ```bash
   portree version
   ```
   If not installed, suggest: `brew install fairy-pitta/tap/portree` (macOS) or download from GitHub releases.

2. **Create `.portree.toml` in the project root:**

   Basic structure:
   ```toml
   [services.SERVICE_NAME]
   command = "COMMAND_TO_START"
   port_env = "PORT"  # environment variable for port
   port_range = [START, END]  # optional, default 3100-3199
   ```

3. **Common configurations:**

   **Node.js/React/Vite:**
   ```toml
   [services.frontend]
   command = "npm run dev"
   port_env = "PORT"
   port_range = [3100, 3199]
   ```

   **Go:**
   ```toml
   [services.api]
   command = "go run ."
   port_env = "PORT"
   port_range = [8100, 8199]
   ```

   **Python/Django:**
   ```toml
   [services.web]
   command = "python manage.py runserver 0.0.0.0:$PORT"
   port_env = "PORT"
   ```

   **Multiple services:**
   ```toml
   [services.frontend]
   command = "npm run dev"
   port_env = "PORT"
   port_range = [3100, 3199]

   [services.backend]
   command = "go run ./cmd/server"
   port_env = "PORT"
   port_range = [8100, 8199]
   ```

4. **Environment variables provided by portree:**
   - `PORT` - Assigned port for this service
   - `PT_BRANCH` - Current branch name
   - `PT_BRANCH_SLUG` - URL-safe branch name (e.g., `feature-auth`)
   - `PT_SERVICE` - Service name
   - `PT_{SERVICE}_PORT` - Port of another service (for cross-service communication)
   - `PT_{SERVICE}_URL` - Proxy URL of another service

5. **Verify setup:**
   ```bash
   portree doctor
   ```

## Example Interaction

User: "I want to set up portree for my Next.js + Express project"

Response: Create `.portree.toml`:
```toml
[services.frontend]
command = "npm run dev"
port_env = "PORT"
port_range = [3100, 3199]

[services.api]
command = "npm run start:api"
port_env = "PORT"
port_range = [8100, 8199]
env = { API_URL = "http://$PT_BRANCH_SLUG.localhost:8000" }
```
