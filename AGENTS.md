# AGENTS.md

## Environment Variables

Sensitive configuration lives in `backend/.env` (git-ignored). See `backend/.env.example` for required variables. The backend justfile auto-loads this file via `set dotenv-load`.

Never commit secrets. Never hardcode credentials in source files.

**Important:** The Spotify API forbids `localhost` in redirect URIs. All local development URLs must use `127.0.0.1` (e.g., `http://127.0.0.1:5173`). Do not refactor these to `localhost`.

## Architecture

Refer to `../project-hub/vibe-seeker/` for architectural documents.

## Testing

- Always run `just test` after changing files in the `backend/` or `frontend/` subdirectories.
- Always run `just test-all` after adding or editing a Type in either the `backend/` or `frontend/` subdirectories.
- Write new unit tests for any testable code you add to the project.  write tests that cover edge cases and error handling.  

## CI

- Always run `just ci` before pushing a commit.
- Always run `just ci` as the last step of any completed task to validate changes.