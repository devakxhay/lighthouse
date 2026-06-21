# Architectural Learning: OpenSSL CA Structure Migration

## Context
Lighthouse was using an ad-hoc signing workflow (`openssl x509 -req` with `-CAcreateserial`). This caused a lack of controlled serial tracking, no permanent audit trail of issued certificates, and no standard revocation/CRL support.

## Solution & Architecture
We migrated the CA implementation to a standardized OpenSSL CA directory structure:
1. **Directories**: `certs/`, `crl/`, `newcerts/`, `private/`.
2. **Metadata Files**: `index.txt` (the flat-file DB tracking all issued/revoked/expired certs), `serial`, and `crlnumber`.
3. **Configuration**: Configured a unified `openssl.cnf` located under the CA directory to control CA behavior.
4. **Command Execution**:
   - Switched from `openssl x509` to `openssl ca`.
   - Used `-batch` to prevent blocking on interactive inputs.
   - Used `-notext` to omit human-readable certificate summaries from the `.crt` file.
   - Used `-revoke` and `-gencrl` to manage certificate revocation and generate CRLs.

## Key Learnings
- **Common Name Conflict**: `openssl ca` refuses to sign a certificate for a domain if there's already an active certificate for that common name in `index.txt` (resulting in `unique_subject` errors). To safely allow re-signing on deployment/update, we must revoke the existing certificate for that domain first if a certificate file already exists.
- **Parsing OpenSSL Dates**: `index.txt` uses UTCTime format (`YYMMDDHHMMSSZ`) or GeneralizedTime format (`YYYYMMDDHHMMSSZ`) for dates. A robust Go parser should inspect the string length and try both layout formats (`060102150405Z` and `20060102150405Z`).
- **Revocation Fields**: In `index.txt`, the third column (revocation timestamp) might include a comma-separated reason (e.g. `260621130600Z,keyCompromise`). It must be split to extract only the timestamp portion for time parsing.
