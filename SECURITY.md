# Security

Found a security problem? Do not open a public issue. Send an email to `contact+security@silical.dev`.

The parts with the most risk are:

- sign-up, passwords and sessions (`services/accounts`)
- the check of published puzzles, which runs an external program on user input (`services/puzzles`)
- the rate limits and the client-address handling behind a proxy
