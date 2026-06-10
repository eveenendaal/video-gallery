# Security Policy

## Supported Versions

Only the latest released version of video-gallery is supported with security
updates. Please upgrade to the most recent release before reporting an issue.

## Reporting a Vulnerability

Please report vulnerabilities privately via
[GitHub Security Advisories](https://github.com/eveenendaal/video-gallery/security/advisories/new)
rather than opening a public issue.

Include as much detail as you can: affected version, reproduction steps, and
potential impact. You can expect an initial response within a week. Once a fix
is available, the vulnerability will be disclosed in the release notes.

## Deployment Security Notes

Access to the gallery and admin pages is controlled entirely by the
`SECRET_KEY` URL prefix. When deploying:

- Use a long, random `SECRET_KEY` (32+ characters). The bundled Terraform
  module generates one automatically.
- Always serve the application behind HTTPS — the secret key is part of every
  URL.
- Treat gallery URLs as capability links: anyone with a link can view that
  gallery and its videos until the signed URLs expire (24 hours). Gallery
  stubs are intentionally short (4 characters) to keep links shareable, so
  they can be discovered by enumerating the /gallery/ namespace.
- The GCS bucket should remain private; the application only exposes content
  through time-limited signed URLs.
