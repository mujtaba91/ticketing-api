# ticketing-api

A REST API for booking event tickets, written in Go with PostgreSQL.

## Design decisions

### Refresh token reuse revokes every session

Refresh tokens rotate: each refresh returns a new token and revokes the old one. If a revoked token is used again, the API assumes it was stolen and increments `users.token_version`, which logs the user out on every device.

**Trade-off: a retry counts as reuse.** If a client sends a refresh request, the response is lost on the network, and the client retries with the same token, the retry looks exactly like an attacker replaying a stolen token. The user is logged out and has to sign in again. Some systems accept the old token for a few seconds after rotation to avoid this. This project keeps the strict rule, because a grace period adds state and a window in which a stolen token still works.

## Left out on purpose

- HttpOnly cookies for browser clients. Tokens are returned in the JSON body, which suits API and mobile clients and keeps testing with curl simple.
