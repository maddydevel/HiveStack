# ADR-0003: Apache License 2.0

## Status

Accepted

## Context

HiveStack is an open-source KVM virtualization management platform. Selecting a license governs:

- How others may use, modify, and distribute the code.
- Whether commercial use and private modifications are permitted.
- Patent protection for contributors and users.
- Compatibility with dependencies.

HiveStack's dependency license survey:

| Dependency | License |
|---|---|
| `golang.org/x/*` | BSD-3-Clause |
| `github.com/jackc/pgx/v5` | MIT |
| `github.com/golang-jwt/jwt/v5` | MIT |
| `github.com/prometheus/client_golang` | Apache-2.0 |
| `github.com/spf13/cobra` | Apache-2.0 |
| `github.com/spf13/viper` | MIT |
| `google.golang.org/grpc` | Apache-2.0 |

All are permissive and compatible with Apache-2.0.

Options considered:

1. **MIT** — Simple, permissive. No explicit patent grant; users could face patent litigation from contributors.
2. **Apache-2.0** — Permissive with explicit patent license from contributors. Compatible with GPLv3 (one-way). Requires preservation of NOTICE file.
3. **GPLv3 / AGPLv3** — Copyleft. Requires derivative works to be open-sourced. AGPL extends to network use. Would limit adoption by commercial users and hosting providers.
4. **Dual-license (Apache-2.0 + commercial)** — Requires contributor license agreements (CLAs) or copyright assignment; overhead for a small project.

## Decision

Use the **Apache License 2.0**.

- Applied to all source code in the HiveStack repository.
- Each `.go` file includes a short SPDX identifier: `// SPDX-License-Identifier: Apache-2.0`
- The `LICENSE` file at the repository root contains the full Apache-2.0 text.
- A `NOTICE` file will be maintained if third-party attribution is required.

## Consequences

### Positive

- Explicit patent license protects users and contributors from patent claims.
- Permissive: allows commercial use, private modification, and redistribution without copyleft obligations.
- Compatible with all current dependencies and with GPLv3 projects (one-way).
- Well-understood by legal teams and the open-source community.

### Negative

- Slightly more complex than MIT (NOTICE file requirements, patent clause language).
- One-way compatible with GPLv3: we can include GPLv3 code in Apache-2.0 projects, but not vice versa.

## References

- `LICENSE` — Full Apache-2.0 license text at repository root.
- `LICENSE` header — Copyright 2026 Maddy AI Consultancy.
- `Creative Commons Legal Code: Apache License 2.0` — https://www.apache.org/licenses/LICENSE-2.0
