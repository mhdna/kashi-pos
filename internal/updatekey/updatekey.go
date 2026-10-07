// Package updatekey holds the public half of the release-signing key. Tills
// only install an update whose signature checks out against it (see
// updater.go), so even a compromised kashi can't push them a program.
//
// `make update-keygen` creates the key pair: the private half stays on the
// developer's machine (~/.config/gb-cloud/update-signing.key -- back it up;
// without it no new update will be accepted) and the public half is written
// here. Empty means remote updates are off in this build.
package updatekey

// PublicKey is a base64 ed25519 public key.
const PublicKey = "RUYn8DqeMZr7dUukaNph52lGVJCLPyANUuQ1xsBQ/qs="
