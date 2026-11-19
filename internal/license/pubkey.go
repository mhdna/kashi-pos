package license

// ServerPublicKeyPEM is the license server's Ed25519 public key. It can only
// check leases, never sign one; the private half lives only on the server
// (kashi-license SIGNING_KEY_FILE). Replacing the server key means shipping a
// build with the new public key here, or every till locks within its offline
// days.
const ServerPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEACcvXTEqwdQ9FfapXYBD9CZnQ19PiQkvyRK64VqoeqUw=
-----END PUBLIC KEY-----
`
