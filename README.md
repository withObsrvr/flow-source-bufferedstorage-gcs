# flow-source-bufferedstorage-gcs

A Flow source plugin that implements the BufferedStorage interface for Google Cloud Storage (GCS).

## Building with Nix

This project uses Nix for reproducible builds.

### Prerequisites

- [Nix package manager](https://nixos.org/download.html) with flakes enabled

### Building

1. Clone the repository:
```bash
git clone https://github.com/withObsrvr/flow-source-bufferedstorage-gcs.git
cd flow-source-bufferedstorage-gcs
```

2. Build with Nix:
```bash
nix build
```

The built plugin will be available at `./result/lib/flow-source-bufferedstorage-gcs.so`.

### Development

To enter a development shell with all dependencies:
```bash
nix develop
```

This will automatically vendor dependencies if needed and provide a shell with all necessary tools.

## Troubleshooting

If you encounter build issues, ensure you have:

1. Enabled flakes in your Nix configuration
2. Properly vendored dependencies with `go mod vendor`
3. Committed all changes (or use `--impure` flag with uncommitted changes)