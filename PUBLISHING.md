# Publishing to Terraform Registry

This guide explains how to publish the `terraform-provider-netbox` to the [Terraform Registry](https://registry.terraform.io/).

## Prerequisites

### 1. GPG Key Setup

The Terraform Registry requires providers to be signed with a GPG key for security.

#### Generate a GPG key (if you don't have one):

```bash
gpg --full-generate-key
```

Choose:
- Key type: RSA and RSA
- Key size: 4096 bits
- Expiration: Choose based on your preference
- Real name and email: Use your GitHub account email

#### Export your GPG key:

```bash
# Get your key ID
gpg --list-secret-keys --keyid-format=long

# Export the private key (keep this secret!)
gpg --armor --export-secret-keys YOUR_KEY_ID > private-key.asc

# Export the public key
gpg --armor --export YOUR_KEY_ID > public-key.asc
```

### 2. GitHub Secrets Configuration

Add the following secrets to your GitHub repository (Settings → Secrets and variables → Actions):

1. **GPG_PRIVATE_KEY**: Content of `private-key.asc`
2. **PASSPHRASE**: Your GPG key passphrase

### 3. Terraform Registry Setup

1. Sign in to [Terraform Registry](https://registry.terraform.io/) with your GitHub account
2. Click "Publish" → "Provider"
3. Select your GitHub repository: `poroping/terraform-provider-netbox`
4. Add your GPG public key:
   - Go to Settings → Signing Keys
   - Upload the content from `public-key.asc`
   - The ASCII-armored key must include the header and footer

## Release Process

### Creating a Release

Releases are automated via GitHub Actions. To create a new release:

1. **Tag the release:**
   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

2. **GitHub Actions will automatically:**
   - Build binaries for all platforms (Linux, macOS, Windows, FreeBSD)
   - Generate checksums
   - Sign the release with your GPG key
   - Create a GitHub release with all artifacts
   - Publish to Terraform Registry (if configured)

### Version Numbering

Follow [Semantic Versioning](https://semver.org/):
- **v0.x.x**: Initial development (breaking changes allowed)
- **v1.0.0**: First stable release
- **MAJOR.MINOR.PATCH**:
  - MAJOR: Breaking changes
  - MINOR: New features (backwards compatible)
  - PATCH: Bug fixes (backwards compatible)

## Terraform Registry Requirements

The Terraform Registry automatically detects new releases from your GitHub repository when:

1. ✅ Repository is public
2. ✅ Repository name format: `terraform-provider-{NAME}`
3. ✅ Tagged releases follow semantic versioning (v1.0.0)
4. ✅ Release artifacts are signed with GPG
5. ✅ Contains proper documentation structure (`docs/`)
6. ✅ Provider uses Terraform Plugin Framework or SDK

## Testing Before Release

Always test before creating a release:

```bash
# Run unit tests
make test

# Run acceptance tests
NETBOX_URL=https://demo.netbox.dev \
NETBOX_TOKEN=your-token \
make testacc

# Build locally
make build

# Install locally for manual testing
make install
```

## Post-Release Verification

After creating a release:

1. Check [GitHub Releases](https://github.com/poroping/terraform-provider-netbox/releases) for artifacts
2. Verify signature: `gpg --verify SHA256SUMS.sig SHA256SUMS`
3. Check [Terraform Registry](https://registry.terraform.io/providers/poroping/netbox) for new version
4. Test installation:
   ```hcl
   terraform {
     required_providers {
       netbox = {
         source  = "poroping/netbox"
         version = "~> 0.1"
       }
     }
   }
   ```

## Troubleshooting

### Release fails with GPG error
- Verify `GPG_PRIVATE_KEY` secret contains the full ASCII-armored key
- Ensure `PASSPHRASE` secret is set correctly
- Check GPG key hasn't expired

### Provider not appearing in Registry
- Ensure repository is public
- Verify GPG public key is uploaded to Registry
- Check release tag follows `v*` pattern
- Wait a few minutes for Registry to sync

### Build fails
- Ensure `go.mod` is up to date (`go mod tidy`)
- Check Go version compatibility
- Verify all tests pass locally

## Documentation

Documentation is automatically generated from:
- `docs/` directory (created by terraform-plugin-docs)
- Examples in `examples/` directory
- Schema descriptions in provider code

Update docs before release:
```bash
go generate ./...
```

## Support

For issues with:
- **Provider functionality**: [GitHub Issues](https://github.com/poroping/terraform-provider-netbox/issues)
- **Terraform Registry**: [HashiCorp Support](https://support.hashicorp.com/)
- **GPG signing**: See [HashiCorp GPG Guide](https://developer.hashicorp.com/terraform/registry/providers/publishing#generating-gpg-signature)
